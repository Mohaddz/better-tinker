"""
Tinker Bridge Server - REST API bridge for Tinker Python SDK

This FastAPI server wraps the Tinker Python SDK, providing a REST API
for the Go CLI to interact with Tinker services.

The Go CLI passes the API key via Authorization header. The bridge uses
this key to initialize the Tinker SDK client.
"""

import os
import threading
from typing import Optional, List, Dict
from datetime import datetime
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Query, Request, Depends
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

# Tinker SDK imports
try:
    import tinker
    TINKER_AVAILABLE = True
except ImportError:
    TINKER_AVAILABLE = False
    print("⚠ Warning: tinker SDK not installed. Running in mock mode.")


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


class UsageStats(BaseModel):
    total_training_runs: int
    total_checkpoints: int
    compute_hours: float
    storage_gb: float


# ============================================================================
# Client Manager - Thread-safe client caching per API key
# ============================================================================

class TinkerClientManager:
    """Caches Tinker SDK clients per API key to avoid re-initialization."""
    
    def __init__(self):
        self._clients: Dict[str, tuple] = {}  # api_key -> (service_client, rest_client)
        self._lock = threading.Lock()
    
    def get_client(self, api_key: str):
        """Get or create a Tinker client for the given API key."""
        if not api_key:
            return None, None
        
        with self._lock:
            if api_key in self._clients:
                return self._clients[api_key]
            
            try:
                os.environ["TINKER_API_KEY"] = api_key
                
                service_client = tinker.ServiceClient()
                rest_client = service_client.create_rest_client()
                
                self._clients[api_key] = (service_client, rest_client)
                return service_client, rest_client
            except Exception as e:
                print(f"✗ Failed to create Tinker client: {e}")
                return None, None
    
    def clear(self):
        """Clear all cached clients."""
        with self._lock:
            self._clients.clear()


client_manager = TinkerClientManager()


# ============================================================================
# FastAPI App Setup
# ============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """App lifespan handler."""
    if not TINKER_AVAILABLE:
        print("⚠ Running in mock mode (tinker SDK not installed)")
    yield
    client_manager.clear()


app = FastAPI(
    title="Tinker Bridge API",
    description="REST API bridge for Tinker Python SDK",
    version="2.0.0",
    lifespan=lifespan
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# ============================================================================
# Dependency Injection - API Key and Client
# ============================================================================

def extract_api_key(request: Request) -> Optional[str]:
    """Extract API key from Authorization header or environment variable."""
    auth_header = request.headers.get("Authorization", "")
    
    if auth_header.startswith("Bearer "):
        return auth_header[7:]
    
    return os.environ.get("TINKER_API_KEY")


def get_rest_client(request: Request):
    """FastAPI dependency that provides a Tinker REST client."""
    if not TINKER_AVAILABLE:
        raise HTTPException(
            status_code=503,
            detail="Tinker SDK not installed. Please install with: pip install tinker"
        )
    
    api_key = extract_api_key(request)
    
    if not api_key:
        raise HTTPException(
            status_code=401,
            detail="API key required. Pass via Authorization header or set TINKER_API_KEY env var."
        )
    
    _, rest_client = client_manager.get_client(api_key)
    
    if rest_client is None:
        raise HTTPException(
            status_code=503,
            detail="Failed to initialize Tinker client. Please check your API key."
        )
    
    return rest_client


# ============================================================================
# Helper functions
# ============================================================================

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
async def health_check(request: Request):
    """Health check endpoint."""
    api_key = extract_api_key(request)
    has_key = bool(api_key)
    
    client_ready = False
    if has_key and TINKER_AVAILABLE:
        _, rest_client = client_manager.get_client(api_key)
        client_ready = rest_client is not None
    
    return {
        "status": "healthy",
        "tinker_sdk": TINKER_AVAILABLE,
        "api_key_provided": has_key,
        "client_ready": client_ready
    }


@app.get("/training_runs", response_model=TrainingRunsResponse)
async def list_training_runs(
    rest_client=Depends(get_rest_client),
    limit: int = Query(default=20, ge=1, le=100),
    offset: int = Query(default=0, ge=0)
):
    """List all training runs with pagination."""
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
async def get_training_run(run_id: str, rest_client=Depends(get_rest_client)):
    """Get details of a specific training run."""
    try:
        future = rest_client.get_training_run(run_id)
        tr = future.result()
        return convert_training_run(tr)
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Training run not found: {run_id}")
        raise HTTPException(status_code=500, detail=f"Failed to get training run: {str(e)}")


@app.get("/training_runs/{run_id}/checkpoints", response_model=CheckpointsResponse)
async def list_checkpoints(run_id: str, rest_client=Depends(get_rest_client)):
    """List checkpoints for a specific training run."""
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
async def list_user_checkpoints(rest_client=Depends(get_rest_client)):
    """List all checkpoints across all training runs."""
    try:
        future = rest_client.list_user_checkpoints()
        response = future.result()
        
        checkpoints = [convert_checkpoint(cp) for cp in response.checkpoints]
        
        return UserCheckpointsResponse(checkpoints=checkpoints)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to list user checkpoints: {str(e)}")


@app.post("/checkpoints/publish", response_model=CheckpointActionResponse)
async def publish_checkpoint(request_body: CheckpointActionRequest, rest_client=Depends(get_rest_client)):
    """Publish a checkpoint to make it public."""
    try:
        future = rest_client.publish_checkpoint_from_tinker_path(request_body.tinker_path)
        future.result()
        
        return CheckpointActionResponse(
            message=f"Checkpoint published successfully: {request_body.tinker_path}",
            success=True
        )
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {request_body.tinker_path}")
        if "already public" in str(e).lower() or "409" in str(e):
            raise HTTPException(status_code=409, detail=f"Checkpoint is already public: {request_body.tinker_path}")
        raise HTTPException(status_code=500, detail=f"Failed to publish checkpoint: {str(e)}")


@app.post("/checkpoints/unpublish", response_model=CheckpointActionResponse)
async def unpublish_checkpoint(request_body: CheckpointActionRequest, rest_client=Depends(get_rest_client)):
    """Unpublish a checkpoint to make it private."""
    try:
        future = rest_client.unpublish_checkpoint_from_tinker_path(request_body.tinker_path)
        future.result()
        
        return CheckpointActionResponse(
            message=f"Checkpoint unpublished successfully: {request_body.tinker_path}",
            success=True
        )
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {request_body.tinker_path}")
        if "already private" in str(e).lower() or "409" in str(e):
            raise HTTPException(status_code=409, detail=f"Checkpoint is already private: {request_body.tinker_path}")
        raise HTTPException(status_code=500, detail=f"Failed to unpublish checkpoint: {str(e)}")


@app.post("/checkpoints/delete", response_model=CheckpointActionResponse)
async def delete_checkpoint_by_path(request_body: CheckpointActionRequest, rest_client=Depends(get_rest_client)):
    """Delete a checkpoint using its tinker path."""
    try:
        future = rest_client.delete_checkpoint_from_tinker_path(request_body.tinker_path)
        future.result()
        
        return CheckpointActionResponse(
            message=f"Checkpoint deleted successfully: {request_body.tinker_path}",
            success=True
        )
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {request_body.tinker_path}")
        raise HTTPException(status_code=500, detail=f"Failed to delete checkpoint: {str(e)}")


@app.delete("/checkpoints/{training_run_id}/{checkpoint_id}")
async def delete_checkpoint(training_run_id: str, checkpoint_id: str, rest_client=Depends(get_rest_client)):
    """Delete a checkpoint by training run ID and checkpoint ID."""
    try:
        future = rest_client.delete_checkpoint(training_run_id, checkpoint_id)
        future.result()
        
        return {"message": f"Checkpoint deleted successfully: {checkpoint_id}"}
    except Exception as e:
        if "not found" in str(e).lower():
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {checkpoint_id}")
        raise HTTPException(status_code=500, detail=f"Failed to delete checkpoint: {str(e)}")


@app.get("/users/usage", response_model=UsageStats)
async def get_usage_stats(rest_client=Depends(get_rest_client)):
    """Get usage statistics for the user."""
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
async def get_checkpoint_archive_url(training_run_id: str, checkpoint_id: str, rest_client=Depends(get_rest_client)):
    """Get download URL for a checkpoint archive."""
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
╔═══════════════════════════════════════════════════════════╗
║              Tinker Bridge Server v2.0                    ║
╠═══════════════════════════════════════════════════════════╣
║  Server: http://{host}:{port:<5}                             ║
║  Docs:   http://{host}:{port}/docs                        ║
║                                                           ║
║  Standalone: set TINKER_API_KEY environment variable      ║
╚═══════════════════════════════════════════════════════════╝
""")
    
    uvicorn.run(app, host=host, port=port)


if __name__ == "__main__":
    main()