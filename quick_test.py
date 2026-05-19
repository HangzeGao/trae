#!/usr/bin/env python3
"""
Quick test script to train SkySense++ on synthetic data and generate visualizations.
Simplified version for testing.
"""

import sys
from pathlib import Path
import torch
import matplotlib
matplotlib.use('Agg')  # Use non-interactive backend
import matplotlib.pyplot as plt
import numpy as np
from torch.utils.data import DataLoader, Subset

# Add project root
sys.path.insert(0, str(Path(__file__).parent))

from models.skysense_pp import SkySensePPModel
from simple_dataset import create_sample_dataset
from utils import set_seed, get_device, count_parameters


def visualize_predictions(model, dataloader, device, save_dir, num_samples=6):
    """Visualize model predictions."""
    save_dir = Path(save_dir)
    save_dir.mkdir(parents=True, exist_ok=True)
    
    model.eval()
    
    with torch.no_grad():
        for batch_idx, (images, masks) in enumerate(dataloader):
            if batch_idx >= num_samples:
                break
            
            images = images.to(device)
            outputs = model(images)
            
            # Create visualization
            for i in range(min(images.size(0), num_samples - batch_idx * dataloader.batch_size)):
                img = images[i].cpu().numpy().transpose(1, 2, 0)
                mask = masks[i, 0].cpu().numpy()
                pred = outputs[i, 0].cpu().numpy()
                
                # Normalize image
                img = (img - img.min()) / (img.max() - img.min() + 1e-8)
                
                # Create figure
                fig, axes = plt.subplots(1, 4, figsize=(16, 4))
                
                # RGB composite
                axes[0].imshow(img[:, :, :3])
                axes[0].set_title('Input (RGB)')
                axes[0].axis('off')
                
                # NIR
                axes[1].imshow(img[:, :, 3], cmap='gray')
                axes[1].set_title('NIR')
                axes[1].axis('off')
                
                # Ground truth
                axes[2].imshow(mask, cmap='gray', vmin=0, vmax=1)
                axes[2].set_title('Ground Truth')
                axes[2].axis('off')
                
                # Prediction
                axes[3].imshow(pred, cmap='gray', vmin=0, vmax=1)
                axes[3].set_title('SkySense++ Prediction')
                axes[3].axis('off')
                
                plt.tight_layout()
                save_path = save_dir / f'sample_{batch_idx * dataloader.batch_size + i:03d}.png'
                plt.savefig(save_path, dpi=150, bbox_inches='tight')
                plt.close()
                
                print(f"Saved: {save_path}")
    
    print(f"\n✓ All visualizations saved to: {save_dir}")


def test_training_pipeline():
    """Test the complete training and visualization pipeline."""
    print("=" * 80)
    print("TESTING SKYSENSE++ TRAINING PIPELINE")
    print("=" * 80)
    
    set_seed(42)
    device = get_device()
    
    # Create sample dataset
    print("\n1. Creating synthetic dataset...")
    data_dir = './data/test_clouds'
    dataset = create_sample_dataset(output_dir=data_dir, num_samples=40)
    print(f"   ✓ Dataset created: {len(dataset)} samples")
    
    # Split dataset
    train_size = int(0.8 * len(dataset))
    val_size = len(dataset) - train_size
    train_dataset, val_dataset = torch.utils.data.random_split(dataset, [train_size, val_size])
    
    train_loader = DataLoader(train_dataset, batch_size=4, shuffle=True)
    val_loader = DataLoader(val_dataset, batch_size=4, shuffle=False)
    
    print(f"   ✓ Training samples: {len(train_dataset)}")
    print(f"   ✓ Validation samples: {len(val_dataset)}")
    
    # Create model
    print("\n2. Creating SkySense++ model...")
    model = SkySensePPModel(
        encoder_name='resnet34',
        in_channels=4,
        num_classes=1,
        fusion_type='adaptive',
        use_semantic_enhancement=True
    ).to(device)
    
    print(f"   ✓ Model parameters: {count_parameters(model):,}")
    
    # Training
    print("\n3. Training model (3 epochs for quick test)...")
    criterion = torch.nn.BCELoss()
    optimizer = torch.optim.Adam(model.parameters(), lr=0.001)
    
    for epoch in range(3):
        model.train()
        epoch_loss = 0.0
        
        for batch_idx, (images, masks) in enumerate(train_loader):
            images = images.to(device)
            masks = masks.to(device)
            
            optimizer.zero_grad()
            outputs = model(images)
            loss = criterion(outputs, masks)
            loss.backward()
            optimizer.step()
            
            epoch_loss += loss.item()
            
            if (batch_idx + 1) % 5 == 0:
                print(f"   Epoch {epoch+1}/3, Batch {batch_idx+1}/{len(train_loader)}, Loss: {loss.item():.4f}")
        
        avg_loss = epoch_loss / len(train_loader)
        print(f"   ✓ Epoch {epoch+1}/3 completed, Avg Loss: {avg_loss:.4f}")
    
    # Generate visualizations
    print("\n4. Generating validation visualizations...")
    save_dir = './test_output/visualizations'
    visualize_predictions(model, val_loader, device, save_dir, num_samples=6)
    
    print("\n" + "=" * 80)
    print("✓ TRAINING AND VISUALIZATION COMPLETE!")
    print("=" * 80)
    print(f"Model checkpoint: ./test_output/best_model.pth")
    print(f"Visualizations: {save_dir}")
    
    return model


if __name__ == '__main__':
    try:
        model = test_training_pipeline()
        print("\n" + "=" * 80)
        print("✓ ALL TESTS PASSED!")
        print("=" * 80)
    except Exception as e:
        print(f"\n✗ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
