import subprocess
import sys
import os
import platform

def main():
    """
    Wrapper to run the bundled Tinker CLI binary.
    """
    system = platform.system().lower()
    is_windows = system == "windows"
    
    # Map OS to binary name
    if is_windows:
        binary_name = "tinker-cli-windows.exe"
    elif system == "darwin": # Mac
        binary_name = "tinker-cli-darwin"
    else: # Linux
        binary_name = "tinker-cli-linux"
    
    # Path to the binary inside the installed package
    current_dir = os.path.dirname(os.path.abspath(__file__))
    binary_path = os.path.join(current_dir, "bin", binary_name)

    # Fallback: check if we are in dev mode and the binary is in the project root
    if not os.path.exists(binary_path):
        # Look 2 levels up (better-tinker root)
        project_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        
        # Check for compiled binaries in root bin/ (if build_binaries.py was run locally)
        local_bin_path = os.path.join(project_root, "better_tinker", "bin", binary_name)
        
        # Check for standard go build output (tinker-cli.exe)
        dev_binary_name = "tinker-cli.exe" if is_windows else "tinker-cli"
        dev_binary_path = os.path.join(project_root, dev_binary_name)

        if os.path.exists(local_bin_path):
            binary_path = local_bin_path
        elif os.path.exists(dev_binary_path):
            binary_path = dev_binary_path
        else:
            # If still not found, try to compile it on the fly (Dev convenience)
            print(f"[*] Tinker binary not found at {binary_path}")
            print("[*] Attempting to compile from source (Dev Mode)...")
            try:
                subprocess.run(["go", "build", "-o", dev_binary_path, "main.go"], cwd=project_root, check=True)
                binary_path = dev_binary_path
                print("[*] Compilation successful.")
            except Exception as e:
                print(f"[!] Error: Could not find or compile binary for {system}")
                print(f"[!] Debug paths checked:\n  {binary_path}\n  {dev_binary_path}")
                sys.exit(1)

    # Ensure executable permissions on Unix
    if not is_windows and os.path.exists(binary_path):
        current_perms = os.stat(binary_path).st_mode
        if not (current_perms & 0o111):
            os.chmod(binary_path, current_perms | 0o111)

    # Run
    try:
        args = [binary_path] + sys.argv[1:]
        # On Windows, using shell=False is safer, but sometimes creating a new console is needed for interactive CLIs.
        # For now, subprocess.run is fine.
        result = subprocess.run(args)
        sys.exit(result.returncode)
    except KeyboardInterrupt:
        sys.exit(130)
    except Exception as e:
        print(f"[!] Error running tinker: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()

