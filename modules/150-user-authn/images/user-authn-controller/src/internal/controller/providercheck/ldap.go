/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package providercheck

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

var _ LDAPConn = (*ldap.Conn)(nil)

type defaultLDAP struct{}

var _ LDAPDialer = defaultLDAP{}

// NewDefaultLDAP returns an LDAPDialer backed by go-ldap.
func NewDefaultLDAP() LDAPDialer {
	return defaultLDAP{}
}

func (defaultLDAP) Dial(ctx context.Context, cfg *DexProviderLDAPForCheck) (LDAPConn, error) {
	return ldapDial(ctx, cfg)
}

func (r *Reconciler) checkLDAP(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.LDAP == nil {
		result.fail("ldapReachable", "LDAP provider config is missing")
		return
	}
	if provider.Spec.LDAP.Host == "" {
		result.fail("ldapReachable", "LDAP host is empty")
		return
	}

	checkCABundle(result, "ldapCABundle", provider.Spec.LDAP.RootCAData)

	conn, err := r.ldap.Dial(ctx, provider.Spec.LDAP)
	if err != nil {
		result.failUnreachable("ldapReachable", err, "LDAP endpoint is not reachable: %v", err)
	} else {
		defer func() { _ = conn.Close() }()
		result.succeed("ldapReachable", "LDAP endpoint is reachable")
	}

	switch {
	case provider.Spec.LDAP.InsecureNoSSL:
		result.skip("ldapCertificate", "TLS is disabled (insecureNoSSL)")
	case err != nil:
		result.skip("ldapCertificate", "skipped because the LDAP endpoint is not reachable")
	case provider.Spec.LDAP.InsecureSkipVerify:
		result.warn("ldapCertificate", "TLS certificate verification is disabled (insecureSkipVerify); the LDAP server certificate is not validated")
	default:
		if state, ok := conn.TLSConnectionState(); ok {
			reportLeafExpiry(result, "ldapCertificate", state.PeerCertificates)
		} else {
			result.fail("ldapCertificate", "TLS connection state is not available")
		}
	}

	checkLDAPBind(result, provider.Spec.LDAP, conn, err)
	r.checkLDAPKerberosKeytab(ctx, result, provider)
}

func checkLDAPBind(result *dexProviderCheckResult, cfg *DexProviderLDAPForCheck, conn LDAPConn, dialErr error) {
	switch {
	case dialErr != nil:
		result.skip("ldapBind", "skipped because the LDAP endpoint is not reachable")
	case cfg.BindDN == "":
		result.skip("ldapBind", "no bindDN configured (anonymous access)")
	default:
		if err := conn.Bind(cfg.BindDN, cfg.BindPW); err != nil {
			result.fail("ldapBind", "LDAP service account bind failed: %v", err)
		} else {
			result.succeed("ldapBind", "LDAP service account bind succeeded")
		}
	}
}

func (r *Reconciler) checkLDAPKerberosKeytab(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.LDAP.Kerberos == nil || !provider.Spec.LDAP.Kerberos.Enabled {
		result.skip("ldapKerberosKeytab", "LDAP Kerberos is disabled")
		return
	}
	if provider.Spec.LDAP.Kerberos.KeytabSecretName == "" {
		result.fail("ldapKerberosKeytab", "LDAP Kerberos is enabled but keytabSecretName is empty")
		return
	}

	secret := controller.Object(controller.SecretGVK)
	err := r.apiReader.Get(ctx, types.NamespacedName{
		Namespace: naming.DexNamespace,
		Name:      provider.Spec.LDAP.Kerberos.KeytabSecretName,
	}, secret)
	if err != nil {
		r.log.Info("cannot find LDAP Kerberos keytab Secret",
			"provider", provider.Name,
			"secret", provider.Spec.LDAP.Kerberos.KeytabSecretName,
		)
		result.fail("ldapKerberosKeytab", "keytab Secret %q is not available", provider.Spec.LDAP.Kerberos.KeytabSecretName)
		return
	}
	result.succeed("ldapKerberosKeytab", "keytab Secret %q is available", provider.Spec.LDAP.Kerberos.KeytabSecretName)
}

func ldapDial(ctx context.Context, cfg *DexProviderLDAPForCheck) (*ldap.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	host, serverName, err := ldapAddress(cfg)
	if err != nil {
		return nil, err
	}

	timeout := ldapTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, ctx.Err()
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	dialOpts := []ldap.DialOpt{ldap.DialWithDialer(&net.Dialer{Timeout: timeout})}

	if cfg.InsecureNoSSL {
		conn, err := ldap.DialURL("ldap://"+host, dialOpts...)
		if err != nil {
			return nil, fmt.Errorf("dial ldap: %w", err)
		}
		if err := closeIfCanceled(ctx, conn); err != nil {
			return nil, err
		}
		conn.SetTimeout(timeout)
		return conn, nil
	}

	tlsConfig, err := buildTLSConfig(cfg.RootCAData, serverName, cfg.InsecureSkipVerify)
	if err != nil {
		return nil, err
	}

	if cfg.StartTLS {
		conn, err := ldap.DialURL("ldap://"+host, dialOpts...)
		if err != nil {
			return nil, fmt.Errorf("dial ldap: %w", err)
		}
		conn.SetTimeout(timeout)
		if err := conn.StartTLS(tlsConfig); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("LDAP StartTLS: %w", err)
		}
		if err := closeIfCanceled(ctx, conn); err != nil {
			return nil, err
		}
		return conn, nil
	}

	conn, err := ldap.DialURL("ldaps://"+host, append(dialOpts, ldap.DialWithTLSConfig(tlsConfig))...)
	if err != nil {
		return nil, fmt.Errorf("dial ldaps: %w", err)
	}
	if err := closeIfCanceled(ctx, conn); err != nil {
		return nil, err
	}
	conn.SetTimeout(timeout)
	return conn, nil
}

func closeIfCanceled(ctx context.Context, conn *ldap.Conn) error {
	if err := ctx.Err(); err != nil {
		_ = conn.Close()
		return err
	}
	return nil
}

func ldapAddress(cfg *DexProviderLDAPForCheck) (string, string, error) {
	host, port, err := net.SplitHostPort(cfg.Host)
	if err == nil {
		return net.JoinHostPort(host, port), host, nil
	}

	if strings.Contains(err.Error(), "missing port in address") {
		port = "636"
		if cfg.InsecureNoSSL || cfg.StartTLS {
			port = "389"
		}
		return net.JoinHostPort(cfg.Host, port), cfg.Host, nil
	}
	return "", "", fmt.Errorf("parse LDAP host %q: %w", cfg.Host, err)
}
