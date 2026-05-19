"""
使用加权采样策略训练云分割模型
"""
import yaml
import torch
import torch.nn as nn
import numpy as np
from pathlib import Path
from tqdm import tqdm
import time

from models.unet import create_model
from models.losses import BCEWithDiceLoss
from models.metrics import SegmentationMetrics
from data.multi_dataset_loader import get_combined_dataloader


def train_weighted():
    """使用加权采样策略训练"""
    
    # 加载配置
    with open('config/config.yaml') as f:
        config = yaml.safe_load(f)
    
    print("=" * 60)
    print("加权采样策略训练")
    print("=" * 60)
    
    # 设备
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"设备: {device}")
    
    # 创建模型
    model_cfg = config['model']
    model = create_model(
        architecture=model_cfg['architecture'],
        encoder_name=model_cfg['encoder_name'],
        encoder_weights=model_cfg['encoder_weights'],
        in_channels=model_cfg['in_channels'],
        classes=model_cfg['out_channels'],
        activation=model_cfg['activation']
    ).to(device)
    
    print(f"模型: {model_cfg['architecture']} + {model_cfg['encoder_name']}")
    print(f"输入通道: {model_cfg['in_channels']}")
    
    # 创建加权采样 DataLoader
    print("\n创建加权采样 DataLoader...")
    print("权重配置: 38cloud=1.0, 95cloud=2.0")
    
    train_loader = get_combined_dataloader(
        dataset_names=["38cloud", "95cloud"],
        strategy="weighted",
        batch_size=config['training']['batch_size'],
        shuffle=False,  # weighted sampler 不需要 shuffle
        num_workers=0,
        augment=True
    )
    
    print(f"训练样本数: {len(train_loader.dataset)}")
    print(f"批次数: {len(train_loader)}")
    
    # 损失函数和优化器
    criterion = BCEWithDiceLoss()
    optimizer = torch.optim.Adam(
        model.parameters(),
        lr=config['training']['learning_rate'],
        weight_decay=config['training']['weight_decay']
    )
    
    # 学习率调度器
    scheduler = torch.optim.lr_scheduler.ReduceLROnPlateau(
        optimizer, mode='min', patience=3, factor=0.5
    )
    
    # 训练循环
    num_epochs = config['training']['epochs']
    best_loss = float('inf')
    
    print("\n开始训练...")
    print("=" * 60)
    
    for epoch in range(num_epochs):
        model.train()
        epoch_loss = 0.0
        
        pbar = tqdm(train_loader, desc=f"Epoch {epoch+1}/{num_epochs}")
        
        for batch_idx, (images, masks) in enumerate(pbar):
            images = images.to(device)
            masks = masks.to(device)
            
            # 前向传播
            outputs = model(images)
            loss = criterion(outputs, masks)
            
            # 反向传播
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            
            epoch_loss += loss.item()
            
            # 更新进度条
            pbar.set_postfix({
                'loss': f'{loss.item():.4f}',
                'avg_loss': f'{epoch_loss/(batch_idx+1):.4f}'
            })
        
        avg_loss = epoch_loss / len(train_loader)
        scheduler.step(avg_loss)
        
        print(f"Epoch {epoch+1}/{num_epochs} - 平均损失: {avg_loss:.4f}")
        
        # 保存最佳模型
        if avg_loss < best_loss:
            best_loss = avg_loss
            checkpoint_path = Path(config['training']['checkpoint_dir'])
            checkpoint_path.mkdir(exist_ok=True)
            torch.save(model.state_dict(), checkpoint_path / 'best_model_weighted.pth')
            print(f"  ✓ 保存最佳模型 (损失: {best_loss:.4f})")
    
    print("\n" + "=" * 60)
    print("训练完成!")
    print(f"最佳损失: {best_loss:.4f}")
    print(f"模型保存于: {checkpoint_path / 'best_model_weighted.pth'}")
    print("=" * 60)
    
    return model


if __name__ == "__main__":
    train_weighted()
