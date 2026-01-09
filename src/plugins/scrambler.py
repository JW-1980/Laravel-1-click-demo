import os

class BaseScrambler:
    def process(self, directory):
        raise NotImplementedError

class NoOpScrambler(BaseScrambler):
    def process(self, directory):
        print("No-op scrambling: Files left untouched.")

class Scrambler(BaseScrambler):
    def process(self, directory):
        print(f"Scrambling files in {directory}...")
        for root, dirs, files in os.walk(directory):
            for file in files:
                if file.endswith(".php"):
                    filepath = os.path.join(root, file)
                    self._scramble_file(filepath)

    def _scramble_file(self, filepath):
        # Mock paid feature: Add a comment to every PHP file
        # ensure we don't accidentally close the tag with ?>
        with open(filepath, 'r+') as f:
            content = f.read()
            f.seek(0, 0)
            f.write("<?php /* SCRAMBLED BY DEMO BUILDER */\n" + content.replace("<?php", "", 1))
