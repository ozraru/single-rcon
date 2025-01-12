package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

//go:embed single-rcon.service.tmpl
var systemdServiceTmplData string

var systemdServiceTmpl = template.Must(template.New("systemd-service").Parse(systemdServiceTmplData))

type systemdServiceParam struct {
	ExecStart        string
	WorkingDirectory string
}

func Install(ctx context.Context, conf *ConfigStruct) {
	log.Print("Installing single rcon...")

	if _, err := os.Stat("/etc/systemd/system"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatal("This system seems not to using systemd. Auto install requires systemd environment.")
		}
	}

	if err := os.Mkdir(conf.Install, 0600); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			log.Panic("Failed to make install directory: ", err)
		}
	}
	selfPath, err := os.Executable()
	if err != nil {
		log.Panic("Failed to locate self path: ", err)
	}
	newProgramPath := filepath.Join(conf.Install, "single-rcon")
	if err := copyFile(ctx, selfPath, newProgramPath, 0700); err != nil {
		log.Panic("Failed to copy binary: ", err)
	}
	if err := copyFile(ctx, filepath.Join(filepath.Dir(selfPath), "client-config.yaml"), filepath.Join(conf.Install, "client-config.yaml"), 0600); err != nil {
		log.Panic("Failed to copy config: ", err)
	}
	file, err := os.OpenFile("/etc/systemd/system/single-rcon.service", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Panic("Failed to open systemd service file: ", err)
	}
	defer file.Close()
	if err := systemdServiceTmpl.Execute(file, systemdServiceParam{
		ExecStart:        newProgramPath + " run",
		WorkingDirectory: conf.Install,
	}); err != nil {
		log.Panic("Failed to write systemd service file: ", err)
	}
	cmd := exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Panic("Failed to run daemon-reload: ", err)
	}
	cmd = exec.Command("systemctl", "enable", "--now", "single-rcon.service")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Panic("Failed to run daemon-reload: ", err)
	}
	log.Print("Install: SUCCESS")
}

func copyFile(ctx context.Context, src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to open destination file: %w", err)
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}
	return nil
}

func Uninstall(ctx context.Context, conf *ConfigStruct) {
	log.Print("Uninstalling single rcon...")

	cmd := exec.Command("systemctl", "disable", "--now", "single-rcon.service")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Panic("Failed to run daemon-reload: ", err)
	}

	if err := os.Remove(filepath.Join(conf.Install, "single-rcon")); err != nil {
		log.Panic("Failed to remove program: ", err)
	}
	if err := os.Remove(filepath.Join(conf.Install, "client-config.yaml")); err != nil {
		log.Panic("Failed to remove config: ", err)
	}
	if err := os.Remove(filepath.Join(conf.Install, "hostkey")); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Panic("Failed to remove hostkey: ", err)
		}
	}
	if err := os.Remove(conf.Install); err != nil {
		log.Panic("Failed to remove install directory: ", err)
	}
	if err := os.Remove("/etc/systemd/system/single-rcon.service"); err != nil {
		log.Panic("Failed to remove systemd service: ", err)
	}
	cmd = exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Panic("Failed to run daemon-reload: ", err)
	}
	log.Print("Uninstall: SUCCESS")
}
