package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func install(cfg config, installerPath string, outputDir string, stdout io.Writer, stderr io.Writer) error {
	installer, err := os.Open(installerPath)
	if err != nil {
		return fmt.Errorf("open installer: %w", err)
	}
	defer func() { _ = installer.Close() }()

	stat, err := installer.Stat()
	if err != nil {
		return fmt.Errorf("stat installer: %w", err)
	}

	sum, err := fileMD5(installer)
	if err != nil {
		return fmt.Errorf("hash installer: %w", err)
	}
	if sum != knownInstallerMD5 && !cfg.quiet {
		_, _ = fmt.Fprintf(stderr, "warning: installer MD5 is %s, expected %s; attempting known offsets anyway\n", sum, knownInstallerMD5)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	for _, entry := range installEntries {
		outputPath, err := installPath(outputDir, entry.name)
		if err != nil {
			return err
		}
		if err := ensureWritableDestination(outputPath, cfg.force); err != nil {
			return err
		}

		if cfg.dryRun {
			_, _ = fmt.Fprintf(stdout, "would write %s (%d bytes)\n", outputPath, entry.size)
			continue
		}

		if err := extractEntry(installer, stat.Size(), entry, outputPath, cfg.force); err != nil {
			return err
		}
		if !cfg.quiet {
			_, _ = fmt.Fprintf(stdout, "wrote %s (%d bytes)\n", outputPath, entry.size)
		}
	}

	if !cfg.dryRun {
		outputPath, err := installPath(outputDir, "Uninstal.exe")
		if err != nil {
			return err
		}
		if err := finalizeUninstaller(outputPath); err != nil {
			return err
		}
		if !cfg.quiet {
			_, _ = fmt.Fprintf(stdout, "finalized %s uninstall metadata\n", outputPath)
		}
	}

	return nil
}

func installPath(root string, entryName string) (string, error) {
	slashed := strings.ReplaceAll(entryName, "\\", "/")
	cleaned := filepath.Clean(filepath.FromSlash(slashed))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe install entry path %q", entryName)
	}
	return filepath.Join(root, cleaned), nil
}

func ensureWritableDestination(path string, force bool) error {
	if _, err := os.Stat(path); err == nil {
		if !force {
			return fmt.Errorf("%s already exists; use -force to overwrite", path)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return nil
}

func fileMD5(file *os.File) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
