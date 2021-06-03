package hooks

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

func generateTestCert() (string, string) {
	priv, _ := rsa.GenerateKey(rand.Reader, 1024)
	keyUsage := x509.KeyUsageDigitalSignature
	template := x509.Certificate{
		SerialNumber: new(big.Int).Lsh(big.NewInt(1), 128),
		Subject: pkix.Name{
			Organization: []string{"Deckhouse test"},
		},
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		panic(err)
	}

	b := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	pp := x509.MarshalPKCS1PrivateKey(priv)
	p := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pp})

	return base64.StdEncoding.EncodeToString(b), base64.StdEncoding.EncodeToString(p)
}

// test helpers

func testHelperSetETCDMembers(members []*etcdserverpb.Member) {
	mems := make([]*etcdserverpb.Member, len(members))
	for i, member := range members {
		a := *member
		mems[i] = &a
	}
	dependency.TestDC.EtcdClient.MemberListMock.Set(func(_ context.Context) (mp1 *clientv3.MemberListResponse, err error) {
		return &clientv3.MemberListResponse{
			Members: mems,
		}, nil
	})
}

func testHelperRegisterEtcdMemberUpdate() {
	dependency.TestDC.EtcdClient.MemberUpdateMock.Set(func(ctx context.Context, id uint64, peers []string) (mp1 *clientv3.MemberUpdateResponse, err error) {
		resp, _ := dependency.TestDC.EtcdClient.MemberList(ctx)
		members := resp.Members
		for i, member := range members {
			if member.ID != id {
				continue
			}
			member.PeerURLs = peers
			members[i] = member
			break
		}

		testHelperSetETCDMembers(members)
		return nil, nil
	})

	dependency.TestDC.EtcdClient.MemberRemoveMock.Set(func(ctx context.Context, id uint64) (mp1 *clientv3.MemberRemoveResponse, err error) {
		resp, _ := dependency.TestDC.EtcdClient.MemberList(ctx)
		members := resp.Members
		var index int
		for i, member := range members {
			if member.ID != id {
				continue
			}
			index = i
			break
		}
		members = append(members[:index], members[index+1:]...)
		testHelperSetETCDMembers(members)

		return &clientv3.MemberRemoveResponse{Members: members}, nil
	})

	dependency.TestDC.EtcdClient.CloseMock.Return(nil)
}
