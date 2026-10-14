// Package config loads Vault Agent Injector's rendered secret file before
// the rest of the app reads its environment.
//
// The usual pattern for consuming a Vault-injected secret is a shell
// wrapper that `source`s /vault/secrets/<name> before exec-ing the real
// process. That doesn't work here: the backend's image is distroless (see
// Dockerfile) specifically so there's no shell to escape into if the
// process is ever compromised - which means there's also no shell
// available to source anything. So the app parses the file itself instead.
//
// No-op if the file doesn't exist - local dev and docker-compose never
// have Vault in the picture, secrets come straight from secrets/backend.env.
package config

import (
	"bufio"
	"os"
	"strings"
)

const vaultSecretFile = "/vault/secrets/backend"

func LoadVaultSecrets() error {
	f, err := os.Open(vaultSecretFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		if err := os.Setenv(strings.TrimSpace(key), strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	return scanner.Err()
}
