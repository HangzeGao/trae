"""
Helper utilities for cloud segmentation project.
"""
import torch
import random
import numpy as np
from pathlib import Path
from typing import Optional, Dict, Any


def set_seed(seed: int = 42):
    """
    Set random seed for reproducibility.
    
    Args:
        seed: Random seed value
    """
    random.seed(seed)
    np.random.seed(seed)
    torch.manual_seed(seed)
    torch.cuda.manual_seed(seed)
    torch.cuda.manual_seed_all(seed)
    torch.backends.cudnn.deterministic = True
    torch.backends.cudnn.benchmark = False


def get_device() -> torch.device:
    """
    Get available device (CUDA, MPS, or CPU).
    Supports Apple Silicon chips.
    
    Returns:
        torch.device object
    """
    if torch.cuda.is_available():
        return torch.device('cuda')
    elif hasattr(torch.backends, 'mps') and torch.backends.mps.is_available():
        return torch.device('mps')
    else:
        return torch.device('cpu')


def count_parameters(model: torch.nn.Module) -> int:
    """
    Count number of trainable parameters in model.
    
    Args:
        model: PyTorch model
    
    Returns:
        Number of trainable parameters
    """
    return sum(p.numel() for p in model.parameters() if p.requires_grad)


def save_checkpoint(
    model: torch.nn.Module,
    optimizer: Optional[torch.optim.Optimizer] = None,
    epoch: int = 0,
    loss: float = 0.0,
    checkpoint_path: Path = Path('./checkpoints'),
    filename: str = 'checkpoint.pth'
):
    """
    Save model checkpoint.
    
    Args:
        model: PyTorch model
        optimizer: Optimizer (optional)
        epoch: Current epoch
        loss: Current loss
        checkpoint_path: Directory to save checkpoint
        filename: Checkpoint filename
    """
    checkpoint_path.mkdir(parents=True, exist_ok=True)
    
    checkpoint = {
        'epoch': epoch,
        'loss': loss,
        'model_state_dict': model.state_dict(),
    }
    
    if optimizer is not None:
        checkpoint['optimizer_state_dict'] = optimizer.state_dict()
    
    torch.save(checkpoint, checkpoint_path / filename)


def load_checkpoint(
    model: torch.nn.Module,
    checkpoint_path: Path,
    optimizer: Optional[torch.optim.Optimizer] = None,
    device: Optional[torch.device] = None
) -> Dict[str, Any]:
    """
    Load model checkpoint.
    
    Args:
        model: PyTorch model
        checkpoint_path: Path to checkpoint file
        optimizer: Optimizer (optional)
        device: Device to load to
    
    Returns:
        Checkpoint dictionary
    """
    if device is None:
        device = get_device()
    
    checkpoint = torch.load(checkpoint_path, map_location=device)
    model.load_state_dict(checkpoint['model_state_dict'])
    
    if optimizer is not None and 'optimizer_state_dict' in checkpoint:
        optimizer.load_state_dict(checkpoint['optimizer_state_dict'])
    
    return checkpoint
