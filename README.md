# Laravel Demo Shipper

This tool allows you to package your Laravel application into a one-click demonstration executable for Windows and Linux (and MacOS with appropriate build tools).

## Features
- **One-Click Single File Demo**: Packages your Laravel app, PHP runtime, and configuration into a single standalone executable. No installation required.
- **Ephemeral Execution**: Runs from a temporary directory and cleans up completely upon exit.
- **Demo Mode Injection**: Sets `IS_DEMO_MODE=true` environment variable so your app can adapt (e.g., hide login, show demo data).
- **Source Scrambling**: Includes a plugin system to obfuscate your source code (Paid feature simulation included).
- **Configurable**: Fully controlled via `manifest.json`.
- **Browser Control**: Attempts to open the demo in "App Mode" (Chrome/Edge) with defined window dimensions.

## Usage

### 1. Configure `manifest.json`
Create a `manifest.json` file in your project root or use the provided template. Key fields:
- `app_name`: Name of your executable.
- `php_port`: Port to run on (0 for random).
- `public_root`: Path to your public folder (relative to the packaged app, e.g., `app/public`).
- `scramble_code`: Set to `true` to enable code scrambling.
- `php_binary_path`: Relative path to the PHP executable within the packaged app (e.g., `php/php.exe`).

### 2. Prepare Dependencies
To include a bundled PHP runtime (required for "run anywhere" capability), place your PHP binaries in a `php/` folder inside your Laravel source directory, or ensure `php_binary_path` points to a valid location relative to the bundle root.

### 3. Build the Demo
Use the Python builder script to package your app.

```bash
# Build for Linux (Current OS)
python3 src/builder/build.py --source /path/to/laravel/project --os linux

# Build for Windows
python3 src/builder/build.py --source /path/to/laravel/project --os windows
```

### 4. Distribute
The output is a single executable file in the `build/` directory (e.g., `laravel_demo` or `laravel_demo.exe`). Send this file to your users. They simply click it to run the demo.

## Plugins
To customize code scrambling, modify `src/plugins/scrambler.py` or provide a custom path in `manifest.json`.

## Requirements
- Go (for compiling the launcher)
- Python 3 (for the builder)
