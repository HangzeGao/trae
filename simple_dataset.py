#!/usr/bin/env python3
"""
Simplified version: Create a basic synthetic cloud dataset with simple file structure.
Uses PNG format for simplicity.
"""

import numpy as np
from PIL import Image
from pathlib import Path
import random
import torch
from torch.utils.data import Dataset
import torchvision.transforms as transforms


class SimpleCloudDataset(Dataset):
    """Simple cloud segmentation dataset."""
    
    def __init__(self, data_dir, num_samples=100, image_size=256, transform=None, train=True):
        """
        Args:
            data_dir: Directory to save/load data
            num_samples: Number of samples to create
            image_size: Size of images
            transform: Optional transform
            train: If True, create new dataset; otherwise load existing
        """
        self.data_dir = Path(data_dir)
        self.image_size = image_size
        self.transform = transform
        self.train = train
        
        if train:
            self.data_dir.mkdir(parents=True, exist_ok=True)
            self._create_dataset(num_samples)
        
        # Load sample list
        self.samples = sorted([f for f in self.data_dir.glob('sample_*')])
        
        if len(self.samples) == 0 and not train:
            raise ValueError(f"No samples found in {data_dir}. Set train=True to create dataset.")
    
    def _create_dataset(self, num_samples):
        """Create synthetic dataset."""
        print(f"Creating synthetic cloud dataset: {num_samples} samples")
        
        for i in range(num_samples):
            # Create random RGB image
            rgb = np.random.randint(50, 200, (self.image_size, self.image_size, 3), dtype=np.uint8)
            
            # Create NIR channel (higher values for vegetation)
            nir = np.random.randint(100, 255, (self.image_size, self.image_size), dtype=np.uint8)
            
            # Combine as 4-channel image
            img_4ch = np.concatenate([
                rgb,
                nir[:, :, np.newaxis]
            ], axis=2)
            
            # Create synthetic cloud mask
            mask = np.zeros((self.image_size, self.image_size), dtype=np.uint8)
            
            # Add random elliptical clouds
            for _ in range(random.randint(2, 5)):
                cx = random.randint(self.image_size//4, 3*self.image_size//4)
                cy = random.randint(self.image_size//4, 3*self.image_size//4)
                rx = random.randint(30, 60)
                ry = random.randint(30, 60)
                
                for y in range(max(0, cy-ry), min(self.image_size, cy+ry)):
                    for x in range(max(0, cx-rx), min(self.image_size, cx+rx)):
                        if ((x-cx)/rx)**2 + ((y-cy)/ry)**2 <= 1:
                            mask[y, x] = 255
            
            # Save as separate PNG files
            sample_dir = self.data_dir / f'sample_{i:04d}'
            sample_dir.mkdir(exist_ok=True)
            
            # Save RGB
            Image.fromarray(rgb).save(sample_dir / 'rgb.png')
            # Save NIR
            Image.fromarray(nir, mode='L').save(sample_dir / 'nir.png')
            # Save mask
            Image.fromarray(mask, mode='L').save(sample_dir / 'mask.png')
            
            if (i + 1) % 20 == 0:
                print(f"  Created {i+1}/{num_samples}")
        
        print(f"✓ Dataset created in {self.data_dir}")
    
    def __len__(self):
        return len(self.samples)
    
    def __getitem__(self, idx):
        sample_dir = self.samples[idx]
        
        # Load image
        rgb = np.array(Image.open(sample_dir / 'rgb.png'))
        nir = np.array(Image.open(sample_dir / 'nir.png'))
        mask = np.array(Image.open(sample_dir / 'mask.png'))
        
        # Stack to 4-channel image
        image = np.concatenate([rgb, nir[:, :, np.newaxis]], axis=2)
        
        # Convert to tensors
        image = torch.from_numpy(image.transpose(2, 0, 1)).float()
        mask = torch.from_numpy(mask).float().unsqueeze(0) / 255.0
        
        if self.transform:
            image = self.transform(image)
        
        return image, mask


def create_sample_dataset(output_dir='./data/sample_clouds', num_samples=50):
    """
    Create a sample dataset for testing.
    
    Returns:
        SimpleCloudDataset instance ready for training
    """
    transform = transforms.Compose([
        transforms.Resize((256, 256)),
    ])
    
    dataset = SimpleCloudDataset(
        data_dir=output_dir,
        num_samples=num_samples,
        transform=transform,
        train=True
    )
    
    return dataset


if __name__ == '__main__':
    print("Creating sample cloud segmentation dataset...")
    dataset = create_sample_dataset(num_samples=50)
    print(f"\nDataset ready: {len(dataset)} samples")
    print(f"Sample: {dataset[0][0].shape}, {dataset[0][1].shape}")
