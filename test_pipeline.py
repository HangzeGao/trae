#!/usr/bin/env python3
"""
Quick test script to train SkySense++ on synthetic data and generate visualizations.
"""

import sys
from pathlib import Path
import torch
import matplotlib.pyplot as plt
import numpy as np

# Add project root
sys.path.insert(0, str(Path(__file__).parent))

from train_skysensepp import train_skysensepp
from simple_dataset import create_sample_dataset


def test_training_pipeline():
    """Test the complete training and visualization pipeline."""
    print("=" * 80)
    print("TESTING SKYSENSE++ TRAINING PIPELINE")
    print("=" * 80)
    
    # Create sample dataset
    print("\n1. Creating synthetic dataset...")
    data_dir = './data/test_clouds'
    dataset = create_sample_dataset(output_dir=data_dir, num_samples=50)
    print(f"   ✓ Dataset created: {len(dataset)} samples")
    
    # Train model
    print("\n2. Training SkySense++ model...")
    print("   (Using 5 epochs for quick testing)")
    
    model, checkpoint_path = train_skysensepp(
        data_dir=data_dir,
        epochs=5,
        batch_size=4,
        learning_rate=0.001,
        save_dir='./test_output',
        val_split=0.2
    )
    
    print("\n3. Training complete!")
    print(f"   ✓ Model saved: {checkpoint_path}")
    print(f"   ✓ Visualizations in: ./test_output/final_visualizations/")
    
    return model


if __name__ == '__main__':
    try:
        test_training_pipeline()
        print("\n" + "=" * 80)
        print("✓ ALL TESTS PASSED!")
        print("=" * 80)
    except Exception as e:
        print(f"\n✗ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
