#!/usr/bin/env python3
"""
Quick training test with simplified model.
"""

import sys
from pathlib import Path
import torch
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from torch.utils.data import DataLoader

sys.path.insert(0, str(Path(__file__).parent))

from simple_skysensepp import SimplifiedSkySensePP
from simple_dataset import create_sample_dataset
from utils import set_seed, get_device


def visualize(model, dataloader, device, save_dir, num_samples=6):
    """Visualize predictions."""
    save_dir = Path(save_dir)
    save_dir.mkdir(parents=True, exist_ok=True)
    
    model.eval()
    
    with torch.no_grad():
        count = 0
        for images, masks in dataloader:
            if count >= num_samples:
                break
            
            images = images.to(device)
            outputs = model(images)
            
            batch_size = images.size(0)
            for i in range(min(batch_size, num_samples - count)):
                img = images[i].cpu().numpy().transpose(1, 2, 0)
                mask = masks[i, 0].cpu().numpy()
                pred = outputs[i, 0].cpu().numpy()
                
                img = (img - img.min()) / (img.max() - img.min() + 1e-8)
                
                fig, axes = plt.subplots(1, 4, figsize=(16, 4))
                
                axes[0].imshow(img[:, :, :3])
                axes[0].set_title('Input (RGB)')
                axes[0].axis('off')
                
                axes[1].imshow(img[:, :, 3], cmap='gray')
                axes[1].set_title('NIR')
                axes[1].axis('off')
                
                axes[2].imshow(mask, cmap='gray', vmin=0, vmax=1)
                axes[2].set_title('Ground Truth')
                axes[2].axis('off')
                
                axes[3].imshow(pred, cmap='gray', vmin=0, vmax=1)
                axes[3].set_title('Prediction')
                axes[3].axis('off')
                
                plt.tight_layout()
                save_path = save_dir / f'val_{count:03d}.png'
                plt.savefig(save_path, dpi=150)
                plt.close()
                
                print(f"Saved: {save_path}")
                count += 1
    
    print(f"\n✓ All visualizations saved to: {save_dir}")


def main():
    print("=" * 80)
    print("TRAINING SIMPLIFIED SKYSENSE++ MODEL")
    print("=" * 80)
    
    set_seed(42)
    device = get_device()
    
    # Create dataset
    print("\n1. Creating dataset...")
    dataset = create_sample_dataset(output_dir='./data/test_clouds', num_samples=40)
    
    train_size = int(0.8 * len(dataset))
    val_size = len(dataset) - train_size
    train_dataset, val_dataset = torch.utils.data.random_split(dataset, [train_size, val_size])
    
    train_loader = DataLoader(train_dataset, batch_size=4, shuffle=True)
    val_loader = DataLoader(val_dataset, batch_size=4, shuffle=False)
    
    print(f"   ✓ Training: {len(train_dataset)}, Validation: {len(val_dataset)}")
    
    # Create model
    print("\n2. Creating model...")
    model = SimplifiedSkySensePP(
        encoder_name='resnet34',
        in_channels=4,
        num_classes=1
    ).to(device)
    
    print(f"   ✓ Model ready")
    
    # Train
    print("\n3. Training (3 epochs)...")
    criterion = torch.nn.BCELoss()
    optimizer = torch.optim.Adam(model.parameters(), lr=0.001)
    
    for epoch in range(3):
        model.train()
        total_loss = 0
        
        for batch_idx, (images, masks) in enumerate(train_loader):
            images = images.to(device)
            masks = masks.to(device)
            
            optimizer.zero_grad()
            outputs = model(images)
            loss = criterion(outputs, masks)
            loss.backward()
            optimizer.step()
            
            total_loss += loss.item()
        
        avg_loss = total_loss / len(train_loader)
        print(f"   Epoch {epoch+1}/3: Loss = {avg_loss:.4f}")
    
    # Visualize
    print("\n4. Generating visualizations...")
    visualize(model, val_loader, device, './test_output/visualizations', num_samples=6)
    
    print("\n" + "=" * 80)
    print("✓ TRAINING COMPLETE!")
    print("=" * 80)
    print("Visualizations saved to: ./test_output/visualizations/")


if __name__ == '__main__':
    main()
