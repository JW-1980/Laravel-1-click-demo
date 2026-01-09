package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	uninstallFlag = flag.Bool("uninstall", false, "Clean up all demo files and exit")
)

func main() {
	flag.Parse()

	// 1. Read Configuration
	manifestPath := "manifest.json"
	// In a real scenario, this might be embedded or next to the executable
	exePath, err := os.Executable()
	if err == nil {
		manifestPath = filepath.Join(filepath.Dir(exePath), "manifest.json")
	}

	// Fallback to current dir if not found (mostly for dev)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		manifestPath = "manifest.json"
	}

	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		fmt.Printf("Error reading manifest: %v\n", err)
		// Try minimal default if manifest fails? No, better to fail.
		os.Exit(1)
	}

	var config Manifest
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("Error parsing manifest: %v\n", err)
		os.Exit(1)
	}

	// 2. Handle Uninstall
	if *uninstallFlag {
		performUninstall(&config)
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
	// Locate PHP binary. In dev, we might use system 'php'.
	// In prod, it should be packaged relative to exe.
	phpBin := config.PHPBinaryPath
	if _, err := os.Stat(phpBin); os.IsNotExist(err) {
		// Fallback to system php
		phpBin = "php"
	} else {
		if filepath.IsAbs(phpBin) == false {
			phpBin = filepath.Join(filepath.Dir(exePath), phpBin)
		}
	}

	publicDir := config.PublicRoot
	if filepath.IsAbs(publicDir) == false {
		publicDir = filepath.Join(filepath.Dir(exePath), publicDir)
	}

	cmd := exec.Command(phpBin, "-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", publicDir)

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
	if config.CleanOnExit {
		// In a real app, this might delete the temp DB or log files
		fmt.Println("Performing cleanup...")
	}
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
	fmt.Println("Uninstalling/Cleaning up demo...")

	// Determine paths relative to executable if needed
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Error determining executable path: %v\n", err)
		return
	}
	exeDir := filepath.Dir(exePath)

	dbPath := config.DBPath
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(exeDir, dbPath)
	}

	fmt.Printf("Removing database at %s...\n", dbPath)
	if err := os.Remove(dbPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Database file does not exist, skipping.")
		} else {
			fmt.Printf("Error removing database: %v\n", err)
		}
	} else {
		fmt.Println("Database removed.")
	}

	// Remove the resources folder if it looks like a demo environment
	resourcesPath := filepath.Join(exeDir, "resources")
	if _, err := os.Stat(resourcesPath); err == nil {
		fmt.Println("Removing resources directory...")
		if err := os.RemoveAll(resourcesPath); err != nil {
			fmt.Printf("Error removing resources: %v\n", err)
		} else {
			fmt.Println("Resources removed.")
		}
	}

	// Additional cleanup could go here (e.g. log files)

	fmt.Println("Cleanup complete.")
}
