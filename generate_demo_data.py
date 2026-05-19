#!/usr/bin/env python3
"""
Generate synthetic cloud segmentation dataset for demonstration purposes.
"""

import numpy as np
from PIL import Image
from pathlib import Path
import pandas as pd


def generate_synthetic_patch(seed, size=256, num_channels=4):
    """Generate a synthetic satellite patch with cloud mask."""
    np.random.seed(seed)
    
    # Generate image: channels - blue, green, red, nir
    img = np.random.rand(num_channels, size, size).astype(np.float32) * 0.5
    
    # Add some structure
    x, y = np.meshgrid(np.linspace(0, 1, size), np.linspace(0, 1, size))
    
    # Create circular cloud
    center_x, center_y = np.random.rand(2)
    radius = 0.2 + 0.3 * np.random.rand()
    cloud_mask = ((x - center_x)**2 + (y - center_y)**2 < radius**2).astype(np.float32)
    
    # Add some random noise to cloud mask
    cloud_mask = cloud_mask * (0.8 + 0.4 * np.random.rand(size, size))
    cloud_mask = np.clip(cloud_mask, 0, 1)
    
    # Add cloud to image (make brighter)
    cloud_brightness = 0.3 + 0.5 * np.random.rand()
    for c in range(min(3, num_channels)):
        img[c] += cloud_mask * cloud_brightness
    
    img = np.clip(img, 0, 1)
    
    # Generate ground truth mask
    gt = (cloud_mask > 0.3).astype(np.float32)
    
    return img, gt


def generate_dataset(output_dir, num_samples=10, patch_size=256):
    """Generate synthetic dataset."""
    output_dir = Path(output_dir)
    train_dir = output_dir / "38-Cloud_training"
    train_dir.mkdir(parents=True, exist_ok=True)
    
    patch_names = []
    
    print(f"Generating {num_samples} synthetic samples...")
    for i in range(num_samples):
        img, gt = generate_synthetic_patch(i, patch_size)
        
        # Save each band
        bands = ["blue", "green", "red", "nir"]
        for c, band in enumerate(bands):
            img_band = (img[c] * 255).astype(np.uint8)
            img_pil = Image.fromarray(img_band)
            img_pil.save(train_dir / f"patch_{i}_{band}.TIF")
        
        # Save ground truth
        gt_img = (gt * 255).astype(np.uint8)
        gt_pil = Image.fromarray(gt_img)
        gt_pil.save(train_dir / f"patch_{i}_gt.TIF")
        
        patch_names.append(f"patch_{i}")
    
    # Create schedule file
    schedule_df = pd.DataFrame({"name": patch_names})
    schedule_df.to_csv(output_dir / "train_patches.csv", index=False)
    
    print(f"Dataset generated at: {output_dir}")
    print(f"Schedule file at: {output_dir / 'train_patches.csv'}")
    
    return str(output_dir / "38-Cloud_training"), str(output_dir / "train_patches.csv")


if __name__ == "__main__":
    data_root = Path(__file__).parent / "data" / "demo_38cloud"
    generate_dataset(data_root, num_samples=50)
