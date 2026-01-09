package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

//go:embed bundle/*
var bundleFS embed.FS

// Manifest matches the structure of manifest.json
type Manifest struct {
	AppName                    string            `json:"app_name"`
	AppVersion                 string            `json:"app_version"`
	WindowWidth                int               `json:"window_width"`
	WindowHeight               int               `json:"window_height"`
	StartMaximized             bool              `json:"start_maximized"`
	PHPPort                    int               `json:"php_port"`
	DBType                     string            `json:"db_type"`
	DBPath                     string            `json:"db_path"`
	EnvVars                    map[string]string `json:"env_vars"`
	DemoModeEnvKey             string            `json:"demo_mode_env_key"`
	SplashScreenImage          string            `json:"splash_screen_image"`
	IconPath                   string            `json:"icon_path"`
	LandingPageURL             string            `json:"landing_page_url"`
	PHPBinaryPath              string            `json:"php_binary_path"`
	PublicRoot                 string            `json:"public_root"`
	ScrambleCode               bool              `json:"scramble_code"`
	ScramblePluginPath         string            `json:"scramble_plugin_path"`
	CleanOnExit                bool              `json:"clean_on_exit"`
	UninstallShortcut          bool              `json:"uninstall_shortcut"`
	AllowedDemoDurationMinutes int               `json:"allowed_demo_duration_minutes"`
}

var (
	uninstallFlag = flag.Bool("uninstall", false, "Clean up any temporary files (automated on exit)")
)

func main() {
	flag.Parse()

	// 0. Extract Bundle to Temp Dir
	tempDir, err := os.MkdirTemp("", "laravel_demo_")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		fmt.Println("Cleaning up...")
		os.RemoveAll(tempDir)
	}()

	// Extract files
	if err := extractBundle(tempDir); err != nil {
		fmt.Printf("Error extracting bundle: %v\n", err)
		os.Exit(1)
	}

	// 1. Read Configuration (from extracted path)
	manifestPath := filepath.Join(tempDir, "bundle", "manifest.json")

	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		fmt.Printf("Error reading manifest: %v\n", err)
		os.Exit(1)
	}

	var config Manifest
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("Error parsing manifest: %v\n", err)
		os.Exit(1)
	}

	// 2. Handle Uninstall (Mostly for show in this ephemeral mode)
	if *uninstallFlag {
		fmt.Println("This application runs from temporary storage and cleans up automatically on exit.")
		// We could force clean common temp patterns if we used a fixed path,
		// but with random temp dirs, there is nothing to 'uninstall' unless running.
		return
	}

	// 3. Find Port
	port := config.PHPPort
	if port == 0 {
		port, err = getFreePort()
		if err != nil {
			fmt.Printf("Error finding free port: %v\n", err)
			os.Exit(1)
		}
	}

	// 4. Start PHP Server
	// Locate PHP binary relative to extracted bundle
	phpBin := filepath.Join(tempDir, "bundle", config.PHPBinaryPath)
	if _, err := os.Stat(phpBin); os.IsNotExist(err) {
		// Fallback to system php
		phpBin = "php"
	}

	publicDir := filepath.Join(tempDir, "bundle", config.PublicRoot)

	// Ensure database exists/is writable if configured inside bundle
	// DBPath in manifest might be "database/database.sqlite" (relative to app)
	// But app is in bundle/app or similar?
	// The manifest says "db_path": "database/database.sqlite".
	// We need to make sure the process runs with CWD that makes sense for Laravel,
	// OR we assume everything is in 'publicDir/../../'

	// Set CWD to the app root (parent of public usually)
	appRoot := filepath.Dir(publicDir)

	cmd := exec.Command(phpBin, "-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", publicDir)
	cmd.Dir = appRoot

	// Inject Env Vars
	env := os.Environ()
	env = append(env, fmt.Sprintf("%s=true", config.DemoModeEnvKey))
	for k, v := range config.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Forward stdout/stderr for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting PHP server: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Server started on http://127.0.0.1:%d\n", port)

	// 5. Open Browser
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, config.LandingPageURL)
	go func() {
		// Give server a moment to start
		time.Sleep(1 * time.Second)
		openBrowser(url, config)
	}()

	// 6. Handle Shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Also handle duration expiry
	if config.AllowedDemoDurationMinutes > 0 {
		go func() {
			time.Sleep(time.Duration(config.AllowedDemoDurationMinutes) * time.Minute)
			fmt.Println("Demo duration expired.")
			c <- os.Interrupt
		}()
	}

	<-c
	fmt.Println("Shutting down...")

	// Kill PHP process
	if err := cmd.Process.Kill(); err != nil {
		fmt.Printf("Error killing server: %v\n", err)
	}

	// 7. Cleanup
	// defer os.RemoveAll(tempDir) handles it.
}

func extractBundle(targetDir string) error {
	// Ensure the root bundle directory exists
	if err := os.MkdirAll(filepath.Join(targetDir, "bundle"), 0755); err != nil {
		return err
	}

	return fs.WalkDir(bundleFS, "bundle", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel("bundle", path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(targetDir, "bundle", relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Ensure parent dir exists (just in case)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		data, err := bundleFS.ReadFile(path)
		if err != nil {
			return err
		}

		return ioutil.WriteFile(destPath, data, 0755)
	})
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func openBrowser(url string, config Manifest) {
	var err error

	// Try to open specific browsers with window size flags if specified
	// This is a "best effort" approach.
	if config.WindowWidth > 0 && config.WindowHeight > 0 {
		switch runtime.GOOS {
		case "windows":
			// Try Chrome
			err = exec.Command("cmd", "/c", "start", "chrome", fmt.Sprintf("--window-size=%d,%d", config.WindowWidth, config.WindowHeight), fmt.Sprintf("--app=%s", url)).Start()
			if err == nil { return }
			// Try Edge
			err = exec.Command("cmd", "/c", "start", "msedge", fmt.Sprintf("--window-size=%d,%d", config.WindowWidth, config.WindowHeight), fmt.Sprintf("--app=%s", url)).Start()
			if err == nil { return }
		case "linux":
			// Try Chrome/Chromium
			err = exec.Command("google-chrome", fmt.Sprintf("--window-size=%d,%d", config.WindowWidth, config.WindowHeight), fmt.Sprintf("--app=%s", url)).Start()
			if err == nil { return }
		}
	}

	// Fallback to default browser
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Printf("Error opening browser: %v\n", err)
	}
}

func performUninstall(config *Manifest) {
	// Deprecated in favor of ephemeral mode
}
