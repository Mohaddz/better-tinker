"""
Tinker Bridge Server - REST API bridge for Tinker Python SDK

This FastAPI server provides a REST API that wraps the Tinker Python SDK,
allowing the Go CLI to interact with Tinker services.
"""

import os
import asyncio
from typing import Optional, List
from datetime import datetime
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Query
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

# Try to import keyring for credential access
try:
    import keyring
except ImportError:
    keyring = None

# Windows-specific credential reading (to match Go's go-keyring format)
def get_api_key_from_windows_credential_manager():
    """
    Read API key from Windows Credential Manager using Go's go-keyring format.
    Go stores: TargetName="tinker-cli", UserName="api-key", CredentialBlob=<password>
    """
    import platform
    if platform.system() != "Windows":
        return None
        
    try:
        import ctypes
        from ctypes import wintypes
        
        # Windows Credential Manager API
        advapi32 = ctypes.windll.advapi32
        
        CRED_TYPE_GENERIC = 1
        
        class CREDENTIAL(ctypes.Structure):
            _fields_ = [
                ("Flags", wintypes.DWORD),
                ("Type", wintypes.DWORD),
                ("TargetName", wintypes.LPWSTR),
                ("Comment", wintypes.LPWSTR),
                ("LastWritten", wintypes.FILETIME),
                ("CredentialBlobSize", wintypes.DWORD),
                ("CredentialBlob", ctypes.POINTER(ctypes.c_ubyte)),
                ("Persist", wintypes.DWORD),
                ("AttributeCount", wintypes.DWORD),
                ("Attributes", ctypes.c_void_p),
                ("TargetAlias", wintypes.LPWSTR),
                ("UserName", wintypes.LPWSTR),
            ]
        
        PCREDENTIAL = ctypes.POINTER(CREDENTIAL)
        
        advapi32.CredReadW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.POINTER(PCREDENTIAL)]
        advapi32.CredReadW.restype = wintypes.BOOL
        advapi32.CredFree.argtypes = [ctypes.c_void_p]
        
        cred_ptr = PCREDENTIAL()
        
        # Go's go-keyring uses "{service}:{username}" as TargetName
        target_name = "tinker-cli:api-key"
        if advapi32.CredReadW(target_name, CRED_TYPE_GENERIC, 0, ctypes.byref(cred_ptr)):
            try:
                cred = cred_ptr.contents
                # Get the password from CredentialBlob
                password_bytes = bytes(cred.CredentialBlob[i] for i in range(cred.CredentialBlobSize))
                return password_bytes.decode('utf-8')
            finally:
                advapi32.CredFree(cred_ptr)
        return None
    except Exception as e:
        print(f"⚠ Windows credential read failed: {e}")
        import traceback
        traceback.print_exc()
        return None

# Tinker SDK imports
try:
    import tinker
    TINKER_AVAILABLE = True
except ImportError:
    TINKER_AVAILABLE = False
    print("Warning: tinker SDK not installed. Running in mock mode.")


# ============================================================================
# Pydantic Models for API responses
# ============================================================================

class LoRAConfig(BaseModel):
    rank: int


class TrainingRun(BaseModel):
    training_run_id: str
    base_model: str
    is_lora: bool
    lora_config: Optional[LoRAConfig] = None
    status: str = "completed"
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None


class Cursor(BaseModel):
    total_count: int
    next_offset: int


class TrainingRunsResponse(BaseModel):
    training_runs: List[TrainingRun]
    cursor: Cursor


class Checkpoint(BaseModel):
    checkpoint_id: str
    name: str
    checkpoint_type: str
    training_run_id: str
    path: str = ""
    tinker_path: str = ""
    is_published: bool = False
    created_at: Optional[datetime] = None
    step: Optional[int] = None


class CheckpointsResponse(BaseModel):
    checkpoints: List[Checkpoint]


class UserCheckpointsResponse(BaseModel):
    checkpoints: List[Checkpoint]


class CheckpointActionRequest(BaseModel):
    tinker_path: str


class CheckpointActionResponse(BaseModel):
    message: str
    success: bool


# Legacy aliases for backwards compatibility
PublishRequest = CheckpointActionRequest
PublishResponse = CheckpointActionResponse


class UsageStats(BaseModel):
    total_training_runs: int
    total_checkpoints: int
    compute_hours: float
    storage_gb: float


class ErrorResponse(BaseModel):
    error: str
    message: str
    code: int


# ============================================================================
# Global client instance
# ============================================================================

service_client = None
rest_client = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize Tinker client on startup."""
    global service_client, rest_client
    
    if TINKER_AVAILABLE:
        api_key = os.environ.get("TINKER_API_KEY")
        
        # Try to get from keyring if not in env
        if not api_key:
            import platform
            current_platform = platform.system()
            
            # On Windows, use direct Windows Credential Manager access
            # (Go's go-keyring stores credentials with TargetName="{service}:{username}")
            if current_platform == "Windows":
                api_key = get_api_key_from_windows_credential_manager()
                if api_key:
                    print("✓ API key retrieved from Windows Credential Manager")
                    os.environ["TINKER_API_KEY"] = api_key
            
            # On macOS/Linux, Python keyring is compatible with Go's go-keyring
            if not api_key and keyring:
                try:
                    api_key = keyring.get_password("tinker-cli", "api-key")
                    if api_key:
                        print("✓ API key retrieved from system keyring")
                        os.environ["TINKER_API_KEY"] = api_key
                except Exception as e:
                    print(f"⚠ Failed to retrieve API key from keyring: {e}")

        if api_key:
            try:
                service_client = tinker.ServiceClient()
                rest_client = service_client.create_rest_client()
                print("✓ Tinker SDK initialized successfully")
            except Exception as e:
                print(f"✗ Failed to initialize Tinker SDK: {e}")
        else:
            print("✗ TINKER_API_KEY not set (checked environment and keyring)")
    else:
        print("⚠ Running in mock mode (tinker SDK not installed)")
    
    yield
    
    # Cleanup
    service_client = None
    rest_client = None


app = FastAPI(
    title="Tinker Bridge API",
    description="REST API bridge for Tinker Python SDK",
    version="1.0.0",
    lifespan=lifespan
)

# Add CORS middleware for local development
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# ============================================================================
# Helper functions
# ============================================================================

def check_client():
    """Check if Tinker client is available."""
    if not TINKER_AVAILABLE:
        raise HTTPException(
            status_code=503,
            detail="Tinker SDK not installed. Please install with: pip install tinker"
        )
    if rest_client is None:
        raise HTTPException(
            status_code=503,
            detail="Tinker client not initialized. Check TINKER_API_KEY environment variable."
        )


def convert_training_run(tr) -> TrainingRun:
    """Convert Tinker SDK training run to our model."""
    lora_config = None
    if hasattr(tr, 'lora_rank') and tr.lora_rank:
        lora_config = LoRAConfig(rank=tr.lora_rank)
    elif hasattr(tr, 'is_lora') and tr.is_lora:
        # Default rank if LoRA but no rank specified
        lora_config = LoRAConfig(rank=32)
    
    return TrainingRun(
        training_run_id=tr.training_run_id if hasattr(tr, 'training_run_id') else str(tr),
        base_model=tr.base_model if hasattr(tr, 'base_model') else "unknown",
        is_lora=tr.is_lora if hasattr(tr, 'is_lora') else False,
        lora_config=lora_config,
        status="completed",
        created_at=tr.created_at if hasattr(tr, 'created_at') else None,
        updated_at=tr.updated_at if hasattr(tr, 'updated_at') else None,
    )


def convert_checkpoint(cp, training_run_id: str = "") -> Checkpoint:
    """Convert Tinker SDK checkpoint to our model."""
    return Checkpoint(
        checkpoint_id=cp.checkpoint_id if hasattr(cp, 'checkpoint_id') else str(cp),
        name=cp.name if hasattr(cp, 'name') else cp.checkpoint_id,
        checkpoint_type=cp.checkpoint_type if hasattr(cp, 'checkpoint_type') else "training",
        training_run_id=cp.training_run_id if hasattr(cp, 'training_run_id') else training_run_id,
        path=cp.path if hasattr(cp, 'path') else "",
        tinker_path=cp.tinker_path if hasattr(cp, 'tinker_path') else "",
        is_published=cp.is_published if hasattr(cp, 'is_published') else False,
        created_at=cp.created_at if hasattr(cp, 'created_at') else None,
        step=cp.step if hasattr(cp, 'step') else None,
    )


# ============================================================================
# API Endpoints
# ============================================================================

@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {
        "status": "healthy",
        "tinker_sdk": TINKER_AVAILABLE,
        "client_initialized": rest_client is not None
    }


@app.get("/training_runs", response_model=TrainingRunsResponse)
async def list_training_runs(
    limit: int = Query(default=20, ge=1, le=100),
    offset: int = Query(default=0, ge=0)
):
    """List all training runs with pagination."""
    check_client()
    
    try:
        future = rest_client.list_training_runs(limit=limit, offset=offset)
        response = future.result()
        
        training_runs = [convert_training_run(tr) for tr in response.training_runs]
        
        return TrainingRunsResponse(
            training_runs=training_runs,
            cursor=Cursor(
                total_count=response.cursor.total_count if hasattr(response.cursor, 'total_count') else len(training_runs),
                next_offset=response.cursor.next_offset if hasattr(response.cursor, 'next_offset') else offset + limit
            )
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to list training runs: {str(e)}")


@app.get("/training_runs/{run_id}", response_model=TrainingRun)
async def get_training_run(run_id: str):
    """Get details of a specific training run."""
    check_client()
    
    try:
        future = rest_client.get_training_run(run_id)
        tr = future.result()
        return convert_training_run(tr)
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Training run not found: {run_id}")
        raise HTTPException(status_code=500, detail=f"Failed to get training run: {str(e)}")


@app.get("/training_runs/{run_id}/checkpoints", response_model=CheckpointsResponse)
async def list_checkpoints(run_id: str):
    """List checkpoints for a specific training run."""
    check_client()
    
    try:
        future = rest_client.list_checkpoints(run_id)
        response = future.result()
        
        checkpoints = [convert_checkpoint(cp, run_id) for cp in response.checkpoints]
        
        return CheckpointsResponse(checkpoints=checkpoints)
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Training run not found: {run_id}")
        raise HTTPException(status_code=500, detail=f"Failed to list checkpoints: {str(e)}")


@app.get("/users/checkpoints", response_model=UserCheckpointsResponse)
async def list_user_checkpoints():
    """List all checkpoints across all training runs."""
    check_client()
    
    try:
        future = rest_client.list_user_checkpoints()
        response = future.result()
        
        checkpoints = [convert_checkpoint(cp) for cp in response.checkpoints]
        
        return UserCheckpointsResponse(checkpoints=checkpoints)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to list user checkpoints: {str(e)}")


@app.post("/checkpoints/publish", response_model=CheckpointActionResponse)
async def publish_checkpoint(request: CheckpointActionRequest):
    """Publish a checkpoint to make it public."""
    check_client()
    
    try:
        future = rest_client.publish_checkpoint_from_tinker_path(request.tinker_path)
        future.result()
        
        return CheckpointActionResponse(
            message=f"Checkpoint published successfully: {request.tinker_path}",
            success=True
        )
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {request.tinker_path}")
        if "already public" in str(e).lower() or "409" in str(e):
            raise HTTPException(status_code=409, detail=f"Checkpoint is already public: {request.tinker_path}")
        raise HTTPException(status_code=500, detail=f"Failed to publish checkpoint: {str(e)}")


@app.post("/checkpoints/unpublish", response_model=CheckpointActionResponse)
async def unpublish_checkpoint(request: CheckpointActionRequest):
    """Unpublish a checkpoint to make it private."""
    check_client()
    
    try:
        future = rest_client.unpublish_checkpoint_from_tinker_path(request.tinker_path)
        future.result()
        
        return CheckpointActionResponse(
            message=f"Checkpoint unpublished successfully: {request.tinker_path}",
            success=True
        )
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {request.tinker_path}")
        if "already private" in str(e).lower() or "409" in str(e):
            raise HTTPException(status_code=409, detail=f"Checkpoint is already private: {request.tinker_path}")
        raise HTTPException(status_code=500, detail=f"Failed to unpublish checkpoint: {str(e)}")


@app.post("/checkpoints/delete", response_model=CheckpointActionResponse)
async def delete_checkpoint_by_path(request: CheckpointActionRequest):
    """Delete a checkpoint using its tinker path."""
    check_client()
    
    try:
        future = rest_client.delete_checkpoint_from_tinker_path(request.tinker_path)
        future.result()
        
        return CheckpointActionResponse(
            message=f"Checkpoint deleted successfully: {request.tinker_path}",
            success=True
        )
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {request.tinker_path}")
        raise HTTPException(status_code=500, detail=f"Failed to delete checkpoint: {str(e)}")


@app.delete("/checkpoints/{training_run_id}/{checkpoint_id}")
async def delete_checkpoint(training_run_id: str, checkpoint_id: str):
    """Delete a checkpoint by training run ID and checkpoint ID."""
    check_client()
    
    try:
        future = rest_client.delete_checkpoint(training_run_id, checkpoint_id)
        future.result()
        
        return {"message": f"Checkpoint deleted successfully: {checkpoint_id}"}
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {checkpoint_id}")
        raise HTTPException(status_code=500, detail=f"Failed to delete checkpoint: {str(e)}")


@app.get("/users/usage", response_model=UsageStats)
async def get_usage_stats():
    """Get usage statistics for the user."""
    check_client()
    
    try:
        # Get training runs count
        tr_future = rest_client.list_training_runs(limit=1)
        tr_response = tr_future.result()
        total_runs = tr_response.cursor.total_count if hasattr(tr_response.cursor, 'total_count') else 0
        
        # Get checkpoints count
        cp_future = rest_client.list_user_checkpoints()
        cp_response = cp_future.result()
        total_checkpoints = len(cp_response.checkpoints) if hasattr(cp_response, 'checkpoints') else 0
        
        # Note: compute_hours and storage_gb might not be available from the SDK
        # These would need a separate API endpoint if available
        return UsageStats(
            total_training_runs=total_runs,
            total_checkpoints=total_checkpoints,
            compute_hours=0.0,  # Not available from current SDK
            storage_gb=0.0  # Not available from current SDK
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to get usage stats: {str(e)}")


@app.get("/checkpoints/{training_run_id}/{checkpoint_id}/archive")
async def get_checkpoint_archive_url(training_run_id: str, checkpoint_id: str):
    """Get download URL for a checkpoint archive."""
    check_client()
    
    try:
        future = rest_client.get_checkpoint_archive_url(training_run_id, checkpoint_id)
        response = future.result()
        
        return {"url": response.url if hasattr(response, 'url') else str(response)}
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found")
        raise HTTPException(status_code=500, detail=f"Failed to get archive URL: {str(e)}")


# ============================================================================
# Main entry point
# ============================================================================

def main():
    import uvicorn
    
    port = int(os.environ.get("TINKER_BRIDGE_PORT", "8765"))
    host = os.environ.get("TINKER_BRIDGE_HOST", "127.0.0.1")
    
    print(f"""
╔══════════════════════════════════════════════════════════════╗
║                    Tinker Bridge Server                       ║
╠══════════════════════════════════════════════════════════════╣
║  Starting server at http://{host}:{port}                      
║  API docs available at http://{host}:{port}/docs              
║                                                               
║  Make sure TINKER_API_KEY is set or saved in CLI settings!    
╚══════════════════════════════════════════════════════════════╝
""")
    
    uvicorn.run(app, host=host, port=port)


if __name__ == "__main__":
    main()

