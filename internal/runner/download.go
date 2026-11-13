package runner

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/system"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// -------------------------------------------------------------
// VERIFICAÇÃO DE INTEGRIDADE (SHA-256)
// -------------------------------------------------------------
func VerifyFileSHA256(filePath, expectedSHA string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("falha ao abrir arquivo para verificação de SHA-256: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("falha ao calcular hash SHA-256: %w", err)
	}

	computed := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(computed, expectedSHA) {
		return fmt.Errorf("checksum mismatch: esperado %s, obtido %s", expectedSHA, computed)
	}
	return nil
}

// downloadArtifact nunca publica ou executa o conteúdo antes da verificação.
// O chamador mantém dir privado e remove-o ao terminar a receita.
func (r *Runner) downloadArtifact(actFn func(string), target *catalog.Target, dir, name, url string) (string, error) {
	if r.dryRun {
		return "", fmt.Errorf("download indisponível em dry-run")
	}
	expected := target.SHA256
	if expected != "" && len(target.ArtifactSHA256) > 0 {
		return "", fmt.Errorf("configure SHA256 ou ArtifactSHA256, não ambos")
	}
	if len(target.ArtifactSHA256) > 0 {
		// Receitas com vários artefatos não reutilizam o hash do artefato principal.
		expected = target.ArtifactSHA256[name]
	}
	if expected != "" {
		decoded, err := hex.DecodeString(expected)
		if err != nil || len(decoded) != sha256.Size {
			return "", fmt.Errorf("SHA-256 inválido para %s", name)
		}
	} else {
		fmt.Fprintf(os.Stderr, "AVISO: %s sem SHA-256 configurado; download sem verificação de integridade.\n", name)
	}
	path := filepath.Join(dir, name)
	curl := "curl"
	if r.sysInfo.OS == system.OSWindows {
		curl = "curl.exe"
	}
	if err := r.execWithActivity(actFn, curl, "--proto", "=https", "--proto-redir", "=https", "-fsSL", url, "-o", path); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("falha ao baixar %s: %w", name, err)
	}
	if expected != "" {
		if err := VerifyFileSHA256(path, expected); err != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("falha de integridade de %s: %w", name, err)
		}
	} else if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
		_ = os.Remove(path)
		return "", fmt.Errorf("download ausente ou vazio: %s", name)
	}
	return path, nil
}

func extractZipStripRoot(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		cleanName := filepath.ToSlash(f.Name)
		parts := strings.Split(cleanName, "/")
		if len(parts) <= 1 {
			continue
		}
		subPath := filepath.Join(parts[1:]...)
		destPath := filepath.Join(destDir, subPath)

		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) && destPath != filepath.Clean(destDir) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(destPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// resolveGitHubLatestTag resolve a tag da release mais recente do GitHub seguindo o redirect
// de /releases/latest, evitando erros de 404 em assets com versão no nome de arquivo.
func resolveGitHubLatestTag(client *http.Client, repo, defaultTag string) string {
	resp, err := client.Head(fmt.Sprintf("https://github.com/%s/releases/latest", repo))
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if idx := strings.LastIndex(loc, "/"); idx != -1 {
				tag := strings.TrimSpace(loc[idx+1:])
				if tag != "" {
					return tag
				}
			}
		}
	}
	return defaultTag
}
