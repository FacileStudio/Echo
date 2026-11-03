package env

import "testing"

func TestIntegrationKeysParseAndRefuseTypos(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	keys, err := ParseIntegrationKeys(" prisme:" + secret + " , bulle:" + secret + "x")
	if err != nil || keys["prisme"] != secret || keys["bulle"] != secret+"x" {
		t.Fatalf("keys %v, err %v", keys, err)
	}
	for _, bad := range []string{"prisme", "Prisme:" + secret, "prisme:short", "a:" + secret + ",a:" + secret} {
		if _, err := ParseIntegrationKeys(bad); err == nil {
			t.Fatalf("%q was accepted", bad)
		}
	}
}

func TestEmbedOriginsMustBeBareOrigins(t *testing.T) {
	origins, err := ParseEmbedOrigins("https://prisme.facile.studio, http://localhost:5173")
	if err != nil || len(origins) != 2 || origins[1] != "http://localhost:5173" {
		t.Fatalf("origins %v, err %v", origins, err)
	}
	for _, bad := range []string{"prisme.facile.studio", "https://prisme.facile.studio/app", "javascript://x"} {
		if _, err := ParseEmbedOrigins(bad); err == nil {
			t.Fatalf("%q was accepted", bad)
		}
	}
}
