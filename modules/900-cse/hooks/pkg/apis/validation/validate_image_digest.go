/*
Copyright 2023 Flant JSC

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

package validation

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/jellydator/ttlcache/v3"
	log "github.com/sirupsen/logrus"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	"go.cypherpunks.ru/gogost/v5/gost34112012256"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	gostHashAnnotationKey       = "gost-digest"
	cacheEvictionDurationSecond = 60 * 5
)

type (
	ImageMetadata struct {
		ImageName       string
		ImageDigest     string
		ImageGostDigest string
		LayersDigest    []string
	}

	validationHandler struct {
		logger                *log.Entry
		registryTransport     *http.Transport
		defaultRegistry       string
		imageHashCache        *ttlcache.Cache[string, string]
		imageMetadataCache    *ttlcache.Cache[string, *ImageMetadata]
		imagePullSecretsCache *ttlcache.Cache[string, struct{}]
		registryAuthCache     *ttlcache.Cache[string, *authn.AuthConfig]
		kubeClient            *kubernetes.Clientset
	}
)

func NewValidationHandler(skipVerify bool) *validationHandler {
	logger := log.WithField("prefix", "image-digest-validation")
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		logger.WithError(err).Warning("can't init kubernetes client in cluster")
	}

	return &validationHandler{
		logger:            logger,
		registryTransport: customTransport,
		defaultRegistry:   name.DefaultRegistry,
		imageHashCache: ttlcache.New[string, string](
			ttlcache.WithTTL[string, string](
				time.Duration(cacheEvictionDurationSecond * time.Second),
			),
		),
		imageMetadataCache: ttlcache.New[string, *ImageMetadata](
			ttlcache.WithTTL[string, *ImageMetadata](
				ttlcache.NoTTL,
			),
		),
		imagePullSecretsCache: ttlcache.New[string, struct{}](
			ttlcache.WithTTL[string, struct{}](
				time.Duration(cacheEvictionDurationSecond * time.Second),
			),
		),
		registryAuthCache: ttlcache.New[string, *authn.AuthConfig](
			ttlcache.WithTTL[string, *authn.AuthConfig](
				ttlcache.NoTTL,
			),
		),
		kubeClient: kubeClient,
	}
}

func newKubeClient() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func (vh *validationHandler) imageDigestValidationHandler() http.Handler {
	vf := kwhvalidating.ValidatorFunc(
		func(ctx context.Context,
			review *model.AdmissionReview,
			obj metav1.Object,
		) (result *kwhvalidating.ValidatorResult, err error) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return rejectResult("incorrect pod data")
			}

			hasAuth, err := vh.updateRegistrySecrets(pod)
			if err != nil {
				return rejectResult(err.Error())
			}
			vh.logger.Debugf("hasAuth, %v", hasAuth)
			for _, image := range vh.GetImagesFromPod(pod) {
				err := vh.CheckImageDigest(image, hasAuth)
				if err != nil {
					return rejectResult(err.Error())
				}
			}
			return allowResult("")
		},
	)

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "image-digest-validation",
		Validator: vf,
		Logger:    validationLogger,
		Obj:       &corev1.Pod{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: validationLogger})
}

func (vh *validationHandler) updateRegistrySecrets(pod *corev1.Pod) (bool, error) {
	if len(pod.Spec.ImagePullSecrets) == 0 {
		vh.logger.Debug("updateRegistrySecrets: ImagePullSecrets empty")
		return false, nil
	}

	for _, secret := range pod.Spec.ImagePullSecrets {
		if vh.imagePullSecretsCache.Has(secret.Name) {
			vh.logger.Debugf("updateRegistrySecrets: imagePullSecretsCache has secret %s, skipped", secret.Name)
			continue
		}

		vh.imagePullSecretsCache.Set(secret.Name, struct{}{}, ttlcache.DefaultTTL)
		vh.logger.Debugf("updateRegistrySecrets: imagePullSecretsCache secret %s added to cache", secret.Name)
		authConfigMap, err := vh.GetAuthConfigsFromSecret(secret.Name, pod.GetNamespace())
		if err != nil {
			vh.logger.WithError(err).Warning("get registry AuthConfig from secret")
			continue
		}

		vh.updateRegistryAuthCache(authConfigMap)
	}

	return true, nil
}

func (vh *validationHandler) updateRegistryAuthCache(authConfigMap map[string]*authn.AuthConfig) {
	for address, authConfig := range authConfigMap {
		vh.logger.WithField(
			"address", address,
		).WithField(
			"authCongig.Username", authConfig.Username,
		).Debug("registryAuthCache: add authConfig to cache")
		vh.registryAuthCache.Set(address, authConfig, ttlcache.NoTTL)
	}
}

func (vh *validationHandler) GetAuthConfigsFromSecret(secretName string, namespace string) (map[string]*authn.AuthConfig, error) {
	result := map[string]*authn.AuthConfig{}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	secret, err := vh.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	vh.logger.Debugf("secret from kube %+v", secret)

	if data, ok := secret.Data[".dockerconfigjson"]; ok {
		var secretData map[string]map[string]*authn.AuthConfig
		err := json.Unmarshal(data, &secretData)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (vh *validationHandler) GetImagesFromPod(pod *corev1.Pod) []string {
	images := []string{}
	for _, container := range pod.Spec.Containers {
		images = append(images, container.Image)
	}
	return images
}

func (vh *validationHandler) GetImageMetadataFromRegistry(imageName string, hasAuth bool) (*ImageMetadata, error) {
	ref, err := vh.ParseImageName(imageName)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	options := []remote.Option{
		remote.WithTransport(vh.registryTransport),
		remote.WithContext(ctx),
	}

	if hasAuth {
		address := ref.Name()
		vh.logger.Debugf("address %s for get authConfig", address)
		authConfigItem := vh.registryAuthCache.Get(address)
		if authConfigItem == nil {
			return nil, fmt.Errorf("can't get authConfig from cache")
		}

		options = append(options, remote.WithAuth(authn.FromConfig(*authConfigItem.Value())))
	}

	image, err := remote.Image(
		ref,
		options...,
	)
	if err != nil {
		return nil, err
	}

	im := &ImageMetadata{ImageName: imageName}
	imageDigest, err := image.Digest()
	if err != nil {
		return nil, err
	}
	im.ImageDigest = imageDigest.String()

	manifest, err := image.Manifest()
	if err != nil {
		return nil, err
	}

	imageGostDigestStr, ok := manifest.Annotations[gostHashAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("the image does not contain gost digest")
	}
	im.ImageGostDigest = imageGostDigestStr

	layers, err := image.Layers()
	if err != nil {
		return nil, err
	}

	for _, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return nil, err
		}
		im.LayersDigest = append(im.LayersDigest, digest.String())
	}

	vh.logger.WithField(
		"imageMetadata", im,
	).Debug("GetImageMetadataFromRegistry")
	return im, nil
}

func (vh *validationHandler) CachedImageMetadata(imageName string) *ImageMetadata {
	imageHashItem := vh.imageHashCache.Get(imageName)
	if imageHashItem == nil {
		vh.logger.WithField("imageName", imageName).Debug("CachedImageMetadata: imageDigest not found")
		return nil
	}

	imageMetadataItem := vh.imageMetadataCache.Get(imageHashItem.Value())
	if imageMetadataItem == nil {
		vh.logger.WithField(
			"imageName", imageName,
		).WithField(
			"imageHash", imageHashItem.Value(),
		).Info("CachedImageMetadata: imageMetadata not found")
		return nil
	}
	im := imageMetadataItem.Value()

	if im == nil {
		vh.logger.WithField(
			"imageName", imageName,
		).WithField(
			"imageHash", imageHashItem.Value(),
		).Warning("CachedImageMetadata: return nil from cache item")
		return nil
	}

	vh.logger.WithField("imageMetadata", *im).Debug("CachedImageMetadata")
	return im
}

func (vh *validationHandler) CacheImageMetadata(im *ImageMetadata) {
	if im == nil {
		vh.logger.Warningf("CacheImageMetadata: image metadata is nil")
		return
	}

	vh.imageHashCache.Set(
		im.ImageName,
		im.ImageDigest,
		ttlcache.DefaultTTL,
	)

	vh.imageMetadataCache.Set(im.ImageDigest, im, ttlcache.NoTTL)
	vh.logger.WithField("imageMetadata", *im).Debug("CacheImageMetadata")
}

func (vh *validationHandler) GetImageMetadata(imageName string, hasAuth bool) (*ImageMetadata, error) {
	if im := vh.CachedImageMetadata(imageName); im != nil {
		return im, nil
	}

	im, err := vh.GetImageMetadataFromRegistry(imageName, hasAuth)
	if err != nil {
		return nil, err
	}

	vh.CacheImageMetadata(im)

	return im, nil
}

func (vh *validationHandler) CheckImageDigest(imageName string, hasAuth bool) error {
	im, err := vh.GetImageMetadata(imageName, hasAuth)
	if err != nil {
		return err
	}

	gostLayersHash, err := vh.CalculateLaersGostHash(im)
	if err != nil {
		return err
	}
	vh.logger.WithField(
		"gostLayersHash", ByteHashToString(gostLayersHash),
	).Debug("image layers gost hash")

	return vh.CompareImageGostHash(im, gostLayersHash)
}

func (vh *validationHandler) ParseImageName(imageName string) (name.Reference, error) {
	return name.ParseReference(imageName, name.WithDefaultRegistry(vh.defaultRegistry))
}

func (vh *validationHandler) CalculateLaersGostHash(im *ImageMetadata) ([]byte, error) {
	layersDigestBuilder := strings.Builder{}
	for _, digest := range im.LayersDigest {
		vh.logger.WithField("layerHash", digest).Debug("image layer hash")
		layersDigestBuilder.WriteString(digest)
	}

	data := layersDigestBuilder.String()

	if len(data) == 0 {
		return nil, fmt.Errorf("invalid layers hash data")
	}

	hasher := gost34112012256.New()
	_, err := hasher.Write([]byte(data))
	if err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}

func (vh *validationHandler) CompareImageGostHash(im *ImageMetadata, gostHash []byte) error {
	imageGostHashByte, err := hex.DecodeString(im.ImageGostDigest)
	if err != nil {
		return fmt.Errorf("invalid gost image digest: %w", err)
	}

	if subtle.ConstantTimeCompare(imageGostHashByte, gostHash) == 0 {
		return fmt.Errorf("invalid gost image digest comparation")
	}
	vh.logger.Debug("CompareImageGostHash success")
	return nil
}

func ByteHashToString(in []byte) string {
	return hex.EncodeToString(in)
}
