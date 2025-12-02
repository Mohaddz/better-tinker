import subprocess
import sys
import os
import platform
import time
import atexit
import signal
import urllib.request
import urllib.error

# Global reference to bridge process for cleanup
_bridge_process = None

BRIDGE_HOST = "127.0.0.1"
BRIDGE_PORT = 8765
BRIDGE_URL = f"http://{BRIDGE_HOST}:{BRIDGE_PORT}"


def is_bridge_running():
    """Check if the bridge server is already running."""
    try:
        req = urllib.request.Request(f"{BRIDGE_URL}/health", method="GET")
        with urllib.request.urlopen(req, timeout=1) as response:
            return response.status == 200
    except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError, OSError):
        return False


def start_bridge():
    """Start the bridge server as a background process."""
    global _bridge_process
    
    # Get the path to the bridge server module
    bridge_module = "better_tinker.bridge.server"
    
    # Start bridge with output suppressed
    _bridge_process = subprocess.Popen(
        [sys.executable, "-m", "uvicorn", 
         "better_tinker.bridge.server:app",
         "--host", BRIDGE_HOST,
         "--port", str(BRIDGE_PORT),
         "--log-level", "warning"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        # On Windows, we need to create a new process group so we can kill it properly
        creationflags=subprocess.CREATE_NEW_PROCESS_GROUP if platform.system() == "Windows" else 0,
    )
    
    return _bridge_process


def wait_for_bridge(timeout=10):
    """Wait for the bridge server to be ready."""
    start_time = time.time()
    while time.time() - start_time < timeout:
        if is_bridge_running():
            return True
        time.sleep(0.1)
    return False


def cleanup_bridge():
    """Terminate the bridge server if we started it."""
    global _bridge_process
    if _bridge_process is not None:
        try:
            if platform.system() == "Windows":
                # On Windows, we need to kill the process tree
                _bridge_process.terminate()
            else:
                # On Unix, send SIGTERM
                _bridge_process.terminate()
            
            # Give it a moment to shut down gracefully
            try:
                _bridge_process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                # Force kill if it doesn't respond
                _bridge_process.kill()
        except Exception:
            pass  # Process might already be dead
        _bridge_process = None


def ensure_bridge():
    """Ensure the bridge is running, starting it if necessary."""
    if is_bridge_running():
        # Bridge already running (maybe from another session)
        return False  # We didn't start it
    
    print("[*] Starting bridge server...")
    start_bridge()
    
    if wait_for_bridge():
        print("[✓] Bridge server ready")
        return True  # We started it
    else:
        print("[!] Failed to start bridge server")
        cleanup_bridge()
        sys.exit(1)


def get_binary_path():
    """Get the path to the tinker CLI binary."""
    system = platform.system().lower()
    is_windows = system == "windows"
    
    # Map OS to binary name
    if is_windows:
        binary_name = "tinker-cli-windows.exe"
    elif system == "darwin":  # Mac
        binary_name = "tinker-cli-darwin"
    else:  # Linux
        binary_name = "tinker-cli-linux"
    
    # Path to the binary inside the installed package
    current_dir = os.path.dirname(os.path.abspath(__file__))
    binary_path = os.path.join(current_dir, "bin", binary_name)

    # Fallback: check if we are in dev mode
    if not os.path.exists(binary_path):
        project_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        
        # Check for compiled binaries in root bin/
        local_bin_path = os.path.join(project_root, "better_tinker", "bin", binary_name)
        
        # Check for standard go build output
        dev_binary_name = "tinker-cli.exe" if is_windows else "tinker-cli"
        dev_binary_path = os.path.join(project_root, dev_binary_name)

        if os.path.exists(local_bin_path):
            binary_path = local_bin_path
        elif os.path.exists(dev_binary_path):
            binary_path = dev_binary_path
        else:
            # Try to compile on the fly (Dev convenience)
            print(f"[*] Tinker binary not found at {binary_path}")
            print("[*] Attempting to compile from source (Dev Mode)...")
            try:
                subprocess.run(["go", "build", "-o", dev_binary_path, "main.go"], 
                             cwd=project_root, check=True)
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
    
    return binary_path


def main():
    """
    Main entry point - starts bridge if needed, runs CLI, cleans up on exit.
    """
    # Register cleanup to run on exit (handles normal exit, exceptions, signals)
    atexit.register(cleanup_bridge)
    
    # Handle signals for clean shutdown
    def signal_handler(signum, frame):
        cleanup_bridge()
        sys.exit(130)
    
    if platform.system() != "Windows":
        signal.signal(signal.SIGINT, signal_handler)
        signal.signal(signal.SIGTERM, signal_handler)
    
    # Ensure bridge is running
    we_started_bridge = ensure_bridge()
    
    # Get the CLI binary
    binary_path = get_binary_path()
    
    # Run the CLI
    try:
        args = [binary_path] + sys.argv[1:]
        result = subprocess.run(args)
        returncode = result.returncode
    except KeyboardInterrupt:
        returncode = 130
    except Exception as e:
        print(f"[!] Error running tinker: {e}")
        returncode = 1
    finally:
        # Always cleanup if we started the bridge
        if we_started_bridge:
            cleanup_bridge()
    
    sys.exit(returncode)


if __name__ == "__main__":
    main()
