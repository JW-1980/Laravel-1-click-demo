# Laravel Demo Shipper

This tool allows you to package your Laravel application into a one-click demonstration executable for Windows and Linux (and MacOS with appropriate build tools).

## Features
- **One-Click Execution**: Bundles a PHP server and launches your app in the default browser.
- **Demo Mode Injection**: Sets `IS_DEMO_MODE=true` environment variable so your app can adapt (e.g., hide login, show demo data).
- **Source Scrambling**: Includes a plugin system to obfuscate your source code (Paid feature simulation included).
- **Configurable**: Fully controlled via `manifest.json`.
- **Uninstall/Cleanup**: Built-in cleanup mechanism.

## Usage

### 1. Configure `manifest.json`
Create a `manifest.json` file in your project root or use the provided template. Key fields:
- `app_name`: Name of your executable.
- `php_port`: Port to run on (0 for random).
- `public_root`: Path to your public folder (relative to the packaged app, usually `resources/app/public`).
- `scramble_code`: Set to `true` to enable code scrambling.
- `php_binary_path`: Relative path to the PHP executable within the packaged app (e.g., `php/php.exe`). You must ensure this binary is available in your source folder or copied during build.

### 2. Build the Demo
Use the Python builder script to package your app.

```bash
# Build for Linux (Current OS)
python3 src/builder/build.py --source /path/to/laravel/project --os linux

# Build for Windows
python3 src/builder/build.py --source /path/to/laravel/project --os windows
```

### 3. Run the Demo
The output will be in the `build/` directory.
- Linux: `./build/laravel_demo`
- Windows: `build\laravel_demo.exe`

## Plugins
To customize code scrambling, modify `src/plugins/scrambler.py` or provide a custom path in `manifest.json`.

## Requirements
- Go (for compiling the launcher)
- Python 3 (for the builder)
