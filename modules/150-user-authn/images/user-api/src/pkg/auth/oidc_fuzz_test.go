func FuzzOIDCVerifier_Verify(f *testing.F) {
	// seed corpus (валидный JWT сценарий)

	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	const keyID = "test-key"
	const publicIssuer = "https://dex.unreachable.invalid/"

	// базовый мок сервер
	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": publicIssuer,
			"jwks_uri": publicIssuer + "keys",
		})
	})

	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       priv.Public(),
			KeyID:     keyID,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}}}
		_ = json.NewEncoder(w).Encode(jwks)
	})

	ts := httptest.NewTLSServer(mux)
	f.Cleanup(ts.Close)

	// seed cases (важно)
	validToken := func() string {
		signer, _ := jose.NewSigner(
			jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: priv, KeyID: keyID}},
			(&jose.SignerOptions{}).WithType("JWT"),
		)

		now := time.Now()

		token, _ := jwt.Signed(signer).Claims(map[string]any{
			"iss":                publicIssuer,
			"aud":                "some-other-client",
			"sub":                "CgR0ZXN0",
			"iat":                now.Unix(),
			"exp":                now.Add(time.Hour).Unix(),
			"preferred_username": "alice",
		}).Serialize()

		return token
	}

	f.Add(validToken())

	// fuzz вход
	f.Fuzz(func(t *testing.T, token string) {
		ctx := context.Background()

		v, err := NewOIDCVerifier(ctx, ts.URL, publicIssuer)
		if err != nil {
			return
		}

		// цель: не падать и не паниковать
		_ = v.Verify(ctx, token)
	})
}
