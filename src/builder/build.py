import json
import os
import shutil
import subprocess
import sys
import importlib.util

class Builder:
    def __init__(self, manifest_path):
        self.manifest_path = manifest_path
        self.config = self.load_manifest()
        self.build_dir = "build"
        # Bundle dir is inside src/launcher for embedding
        self.bundle_dir = os.path.join("src", "launcher", "bundle")
        self.app_dir = os.path.join(self.bundle_dir, "app")

    def load_manifest(self):
        with open(self.manifest_path, 'r') as f:
            return json.load(f)

    def clean_build(self):
        if os.path.exists(self.build_dir):
            shutil.rmtree(self.build_dir)
        os.makedirs(self.build_dir)

        # Clean bundle dir (except .gitignore)
        if os.path.exists(self.bundle_dir):
            for item in os.listdir(self.bundle_dir):
                if item == ".gitignore": continue
                path = os.path.join(self.bundle_dir, item)
                if os.path.isdir(path):
                    shutil.rmtree(path)
                else:
                    os.remove(path)
        else:
            os.makedirs(self.bundle_dir)

        os.makedirs(self.app_dir)

    def copy_source(self, source_path):
        print(f"Copying source from {source_path} to {self.app_dir}...")
        # Ignore .git, build, etc.
        shutil.copytree(source_path, self.app_dir, dirs_exist_ok=True, ignore=shutil.ignore_patterns('.git', 'build', 'venv', '__pycache__'))

        # Copy PHP if available
        # Check source for 'php' folder or config
        php_src = os.path.join(source_path, "php")
        if os.path.exists(php_src):
            print("Copying bundled PHP runtime...")
            shutil.copytree(php_src, os.path.join(self.bundle_dir, "php"), dirs_exist_ok=True)

    def apply_scrambling(self):
        if not self.config.get('scramble_code', False):
            print("Scrambling disabled.")
            return

        plugin_path = self.config.get('scramble_plugin_path')
        if not plugin_path or not os.path.exists(plugin_path):
            print(f"Scramble plugin not found at {plugin_path}")
            return

        print(f"Applying scrambling using {plugin_path}...")

        # Load plugin dynamically
        spec = importlib.util.spec_from_file_location("scrambler_plugin", plugin_path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)

        # Assume plugin has a class 'Scrambler' with method 'process(directory)'
        if hasattr(module, 'Scrambler'):
            scrambler = module.Scrambler()
            scrambler.process(self.app_dir)
        else:
            print("Plugin does not have 'Scrambler' class.")

    def compile_launcher(self, target_os="linux"):
        print(f"Compiling launcher for {target_os}...")

        env = os.environ.copy()
        env["GOOS"] = target_os
        env["GOARCH"] = "amd64"

        output_name = self.config.get('app_name', 'demo').replace(" ", "_").lower()
        if target_os == "windows":
            output_name += ".exe"

        output_path = os.path.join(self.build_dir, output_name)

        # Handle Icon Embedding (Best Effort)
        # Real icon embedding requires 'rsrc' or 'windres' to generate a .syso file.
        # Since we cannot guarantee these tools exist in the user's environment,
        # we check for them or skip with a warning.
        icon_path = self.config.get('icon_path')
        if target_os == "windows" and icon_path and os.path.exists(icon_path):
            print(f"Note: Icon at {icon_path} defined. To embed this icon, ensure 'rsrc' tool is installed.")
            # In a full implementation, we would call: rsrc -manifest ... -ico icon.ico -o src/launcher/rsrc.syso
            # For now, we proceed without embedding to ensure build success.

        cmd = ["go", "build", "-o", output_path, "src/launcher/main.go"]

        try:
            subprocess.check_call(cmd, env=env)
            print(f"Launcher compiled to {output_path}")
        except subprocess.CalledProcessError as e:
            print(f"Compilation failed: {e}")
            sys.exit(1)

    def bundle_config(self):
        # Copy manifest to bundle dir for embedding
        shutil.copy(self.manifest_path, os.path.join(self.bundle_dir, "manifest.json"))

    def build(self, source_path, target_os="linux"):
        self.clean_build()
        self.copy_source(source_path)
        self.apply_scrambling()
        self.bundle_config()
        self.compile_launcher(target_os)
        # Note: Uninstall script generation is less relevant for single-file exe
        # but we can still generate it if needed. For now, skipping as per "Just click exe" philosophy.
        print("Build complete.")

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description="Build Laravel Demo")
    parser.add_argument("--source", required=True, help="Path to Laravel source code")
    parser.add_argument("--manifest", default="manifest.json", help="Path to manifest.json")
    parser.add_argument("--os", default="linux", choices=["linux", "windows", "darwin"], help="Target OS")

    args = parser.parse_args()

    builder = Builder(args.manifest)
    builder.build(args.source, args.os)
