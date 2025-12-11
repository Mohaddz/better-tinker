#!/usr/bin/env python3
"""
Build script for better-tinker Go binaries.

This script cross-compiles the Go CLI for all supported platforms
and optionally code-signs macOS binaries to reduce Keychain prompts.
"""

import subprocess
import os
import sys
import platform
import shutil


def codesign_macos_binary(binary_path: str) -> bool:
    """
    Ad-hoc code sign a macOS binary.
    
    This reduces the number of Keychain password prompts because
    macOS trusts signed binaries more than unsigned ones.
    
    Returns True if signing succeeded, False otherwise.
    """
    if platform.system() != "Darwin":
        print(f"   ℹ Skipping code signing (not on macOS)")
        return False
    
    if not os.path.exists(binary_path):
        print(f"   ⚠ Binary not found: {binary_path}")
        return False
    
    try:
        # Ad-hoc signing (free, no Apple Developer account needed)
        # -s - means ad-hoc signing
        # -f forces re-signing if already signed
        result = subprocess.run(
            ["codesign", "-s", "-", "-f", binary_path],
            capture_output=True,
            text=True
        )
        
        if result.returncode == 0:
            print(f"   ✓ Code signed: {binary_path}")
            return True
        else:
            print(f"   ⚠ Code signing failed: {result.stderr}")
            return False
    except FileNotFoundError:
        print(f"   ⚠ codesign tool not found (expected on macOS)")
        return False
    except Exception as e:
        print(f"   ⚠ Code signing error: {e}")
        return False


def build_all():
    """Build Go binaries for all platforms."""
    # Target directory inside the package
    bin_dir = os.path.join("better_tinker", "bin")
    os.makedirs(bin_dir, exist_ok=True)
    
    platforms = [
        ("windows", "amd64", "tinker-cli-windows.exe"),
        ("linux", "amd64", "tinker-cli-linux"),
        ("darwin", "amd64", "tinker-cli-darwin"),      # Mac Intel
        ("darwin", "arm64", "tinker-cli-darwin-arm64"), # Mac M1/M2/M3
    ]

    print(f"Building Go binaries into {bin_dir}...")
    print()
    
    current_os = platform.system().lower()
    built_binaries = []
    
    for os_name, arch, output_name in platforms:
        print(f"→ Building for {os_name}/{arch}...")
        env = os.environ.copy()
        env["GOOS"] = os_name
        env["GOARCH"] = arch
        env["CGO_ENABLED"] = "0"  # Disable CGO for cross-compilation
        
        output_path = os.path.join(bin_dir, output_name)
        
        try:
            result = subprocess.run(
                ["go", "build", "-ldflags=-s -w", "-o", output_path, "main.go"], 
                env=env, 
                check=True,
                capture_output=True,
                text=True
            )
            print(f"   ✓ Built: {output_path}")
            built_binaries.append((os_name, arch, output_path))
        except subprocess.CalledProcessError as e:
            print(f"   ✗ Failed to build for {os_name}/{arch}")
            if e.stderr:
                print(f"      {e.stderr[:200]}")
        except FileNotFoundError:
            print(f"   ✗ Go compiler not found. Please install Go.")
            sys.exit(1)
        except Exception as e:
            print(f"   ✗ Error: {e}")
    
    print()
    
    # Code sign macOS binaries if we're on macOS
    if current_os == "darwin":
        print("Code signing macOS binaries...")
        for os_name, arch, binary_path in built_binaries:
            if os_name == "darwin":
                codesign_macos_binary(binary_path)
        print()
    else:
        print("ℹ Note: macOS binaries are not code-signed (not building on macOS)")
        print("  To reduce Keychain prompts, build on macOS or manually sign with:")
        print("  codesign -s - better_tinker/bin/tinker-cli-darwin*")
        print()
    
    print("Build process finished.")
    print()
    
    # Summary
    print("Built binaries:")
    for os_name, arch, binary_path in built_binaries:
        size = os.path.getsize(binary_path) / (1024 * 1024)  # MB
        print(f"  • {binary_path} ({size:.1f} MB)")


if __name__ == "__main__":
    build_all()
