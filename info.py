#!/usr/bin/env python3
"""
Cloud Segmentation Project Info

Displays project structure and configuration.
"""

import sys
from pathlib import Path

# Add project root to path
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from utils import load_config
from data.multi_dataset_loader import print_dataset_summary


def main():
    print("=" * 80)
    print("CLOUD SEGMENTATION PROJECT")
    print("=" * 80)
    
    print("\nProject Structure:")
    print("-" * 80)
    
    # Show main directories
    main_dirs = [
        "config/",
        "data/",
        "models/", 
        "utils/",
        "scripts/",
        "checkpoints/"
    ]
    
    for d in main_dirs:
        if Path(d).exists():
            print(f"  ✓ {d}")
        else:
            print(f"  ✗ {d} (missing)")
    
    # Load and display config
    print("\nConfiguration (from config/config.yaml):")
    print("-" * 80)
    
    try:
        config = load_config()
        
        print(f"\nModel:")
        print(f"  Architecture: {config.model.get('architecture')}")
        print(f"  Encoder: {config.model.get('encoder_name')}")
        print(f"  Input channels: {config.model.get('in_channels')}")
        print(f"  Output channels: {config.model.get('out_channels')}")
        
        print(f"\nTraining:")
        print(f"  Epochs: {config.training.get('epochs')}")
        print(f"  Batch size: {config.training.get('batch_size')}")
        print(f"  Learning rate: {config.training.get('learning_rate')}")
        print(f"  Loss: {config.training.get('loss')}")
        
        print(f"\nDatasets:")
        print_dataset_summary(config)
        
        print(f"\nSampling:")
        print(f"  Default strategy: {config.sampling.get('default_strategy')}")
        print(f"  Sample size (quick): {config.sampling.get('sample_size')}")
        
    except Exception as e:
        print(f"  Error loading config: {e}")
    
    print("\nQuick Start:")
    print("-" * 80)
    print("  Train (weighted strategy):  python train.py --strategy weighted")
    print("  Quick validation train:      python train.py --strategy sample --quick")
    print("  Validate:                    python validate.py --checkpoint checkpoints/best_model_weighted.pth")
    print("\n" + "=" * 80)


if __name__ == "__main__":
    main()
