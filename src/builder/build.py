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
        self.resources_dir = os.path.join(self.build_dir, "resources")
        self.app_dir = os.path.join(self.resources_dir, "app")

    def load_manifest(self):
        with open(self.manifest_path, 'r') as f:
            return json.load(f)

    def clean_build(self):
        if os.path.exists(self.build_dir):
            shutil.rmtree(self.build_dir)
        os.makedirs(self.app_dir)

    def copy_source(self, source_path):
        print(f"Copying source from {source_path} to {self.app_dir}...")
        # Ignore .git, build, etc.
        shutil.copytree(source_path, self.app_dir, dirs_exist_ok=True, ignore=shutil.ignore_patterns('.git', 'build', 'venv', '__pycache__'))

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

        cmd = ["go", "build", "-o", output_path, "src/launcher/main.go"]

        try:
            subprocess.check_call(cmd, env=env)
            print(f"Launcher compiled to {output_path}")
        except subprocess.CalledProcessError as e:
            print(f"Compilation failed: {e}")
            sys.exit(1)

    def bundle_config(self):
        # Copy manifest to build dir so launcher can read it
        shutil.copy(self.manifest_path, os.path.join(self.build_dir, "manifest.json"))

    def build(self, source_path, target_os="linux"):
        self.clean_build()
        self.copy_source(source_path)
        self.apply_scrambling()
        self.compile_launcher(target_os)
        self.bundle_config()
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
