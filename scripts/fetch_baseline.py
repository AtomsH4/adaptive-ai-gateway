"""Fetch the pinned macOS arm64 lifecycle baseline into ignored .local/."""
import hashlib
import json
from pathlib import Path
import platform
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
DEST = ROOT / ".local"
ENGINE_URL = "https://github.com/ggml-org/llama.cpp/releases/download/b12110/llama-b12110-bin-macos-arm64.tar.gz"
ENGINE_SHA = "4f5b8de12e0b9516658ca6a5764c6ffb3d7c659c3dd538556b1413c7b4050662"
MODEL_URL = "https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/9217f5db79a29953eb74d5343926648285ec7e67/qwen2.5-0.5b-instruct-q4_k_m.gguf"
MODEL_SHA = "74a4da8c9fdbcd15bd1f6d01d621410d31c6fc00986f5eb687824e7b93d7a9db"


def fetch(url, path, digest, size):
    if path.exists() and hashlib.file_digest(path.open("rb"), "sha256").hexdigest() == digest:
        return
    temporary = path.with_suffix(path.suffix + ".part")
    request = urllib.request.Request(url, headers={"User-Agent": "adaptive-ai-gateway-baseline"})
    downloaded = 0
    try:
        with urllib.request.urlopen(request, timeout=30) as response, temporary.open("wb") as output:
            while data := response.read(1024 * 1024):
                downloaded += len(data)
                if downloaded > size:
                    raise ValueError("download exceeds pinned size")
                output.write(data)
        with temporary.open("rb") as file:
            actual = hashlib.file_digest(file, "sha256").hexdigest()
        if downloaded != size or actual != digest:
            raise ValueError("artifact size or digest mismatch")
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


if __name__ == "__main__":
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        raise SystemExit("This baseline artifact is pinned specifically for macOS arm64.")
    DEST.mkdir(exist_ok=True)
    fetch(ENGINE_URL, DEST / "engine.tar.gz", ENGINE_SHA, 16650691)
    target = DEST / "engine"
    target.mkdir(exist_ok=True)
    with tarfile.open(DEST / "engine.tar.gz") as archive:
        archive.extractall(target, filter="data")
    print("Engine archive verified and extracted.", flush=True)
    fetch(MODEL_URL, DEST / "validation-model.gguf", MODEL_SHA, 491400032)
    print("Model verified; artifacts remain outside tracked source.", flush=True)
