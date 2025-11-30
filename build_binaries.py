import subprocess
import os
import shutil

def build_all():
    # Target directory inside the package
    bin_dir = os.path.join("better_tinker", "bin")
    os.makedirs(bin_dir, exist_ok=True)
    
    platforms = [
        ("windows", "amd64", "tinker-cli-windows.exe"),
        ("linux", "amd64", "tinker-cli-linux"),
        ("darwin", "amd64", "tinker-cli-darwin"), # Mac Intel
        ("darwin", "arm64", "tinker-cli-darwin-arm64"), # Mac M1/M2
    ]

    print(f"Building Go binaries into {bin_dir}...")
    
    for os_name, arch, output_name in platforms:
        print(f"-> Building for {os_name}/{arch}...")
        env = os.environ.copy()
        env["GOOS"] = os_name
        env["GOARCH"] = arch
        
        output_path = os.path.join(bin_dir, output_name)
        
        try:
            subprocess.run(
                ["go", "build", "-o", output_path, "main.go"], 
                env=env, 
                check=True
            )
        except subprocess.CalledProcessError:
            print(f"X Failed to build for {os_name}/{arch}")
            # Don't return, try to build others
        except Exception as e:
            print(f"X Error: {e}")

    print("Build process finished.")

if __name__ == "__main__":
    build_all()

