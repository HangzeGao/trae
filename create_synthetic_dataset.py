#!/usr/bin/env python3
"""
Create a sample synthetic dataset for testing SkySense++ training pipeline.
"""

import numpy as np
from PIL import Image
from pathlib import Path
import random

def create_synthetic_cloud_dataset(output_dir, num_samples=50, image_size=(256, 256)):
    """
    Create synthetic cloud segmentation dataset for testing.
    
    Args:
        output_dir: Output directory
        num_samples: Number of samples to create
        image_size: Size of images (height, width)
    """
    output_dir = Path(output_dir)
    images_dir = output_dir / 'images'
    images_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"Creating synthetic cloud dataset in {output_dir}")
    print(f"Number of samples: {num_samples}")
    print(f"Image size: {image_size}")
    
    for i in range(num_samples):
        # Create random RGB image
        rgb = np.random.randint(0, 256, (image_size[0], image_size[1], 3), dtype=np.uint8)
        
        # Create NIR channel (simulated)
        nir = np.random.randint(0, 256, (image_size[0], image_size[1]), dtype=np.uint8)
        
        # Stack RGB + NIR
        img_data = np.concatenate([
            rgb,
            nir[:, :, np.newaxis]
        ], axis=2)
        
        # Create synthetic cloud mask (elliptical clouds)
        mask = np.zeros(image_size, dtype=np.uint8)
        
        # Add random elliptical clouds
        num_clouds = random.randint(2, 5)
        for _ in range(num_clouds):
            # Random center
            cx = random.randint(image_size[1]//4, 3*image_size[1]//4)
            cy = random.randint(image_size[0]//4, 3*image_size[0]//4)
            
            # Random radius
            rx = random.randint(20, 50)
            ry = random.randint(20, 50)
            
            # Draw ellipse
            for y in range(max(0, cy-ry), min(image_size[0], cy+ry)):
                for x in range(max(0, cx-rx), min(image_size[1], cx+rx)):
                    if ((x-cx)/rx)**2 + ((y-cy)/ry)**2 <= 1:
                        mask[y, x] = 255
        
        # Save RGB bands
        for band_idx, band in enumerate(['red', 'green', 'blue', 'nir']):
            band_data = img_data[:, :, band_idx]
            band_img = Image.fromarray(band_data, mode='L')
            band_img.save(images_dir / f'patch_{i:04d}_{band}.TIF')
        
        # Save mask
        mask_img = Image.fromarray(mask, mode='L')
        mask_img.save(images_dir / f'patch_{i:04d}_gt.TIF')
        
        if (i + 1) % 10 == 0:
            print(f"Created {i+1}/{num_samples} samples")
    
    print(f"\n✓ Synthetic dataset created successfully!")
    print(f"Location: {images_dir}")
    print(f"Total samples: {num_samples}")
    
    # Print sample file names
    sample_files = list(images_dir.glob('*.TIF'))[:5]
    print(f"\nSample files:")
    for f in sample_files:
        print(f"  - {f.name}")
    
    return str(images_dir)


if __name__ == '__main__':
    import argparse
    
    parser = argparse.ArgumentParser(description='Create synthetic cloud dataset')
    parser.add_argument('--output_dir', type=str, default='./data/synthetic_clouds', help='Output directory')
    parser.add_argument('--num_samples', type=int, default=50, help='Number of samples')
    parser.add_argument('--image_size', type=int, nargs=2, default=[256, 256], help='Image size (H W)')
    
    args = parser.parse_args()
    
    data_dir = create_synthetic_cloud_dataset(
        output_dir=args.output_dir,
        num_samples=args.num_samples,
        image_size=tuple(args.image_size)
    )
    
    print(f"\n{'=' * 60}")
    print(f"You can now train SkySense++ with:")
    print(f"python train_skysensepp.py --data_dir {data_dir} --epochs 10")
    print(f"{'=' * 60}")
