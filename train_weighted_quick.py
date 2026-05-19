"""
使用加权采样策略快速训练（使用部分数据）
"""
import yaml
import torch
import numpy as np
from pathlib import Path
from tqdm import tqdm

from models.unet import create_model
from models.losses import BCEWithDiceLoss
from models.metrics import SegmentationMetrics
from data.multi_dataset_loader import get_combined_dataloader, CloudSegmentationDataset
from torch.utils.data import Subset


def train_weighted_quick():
    """使用加权采样策略快速训练"""
    
    # 加载配置
    with open('config/config.yaml') as f:
        config = yaml.safe_load(f)
    
    print("=" * 60)
    print("加权采样策略快速训练")
    print("=" * 60)
    
    # 设备
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"设备: {device}")
    
    # 创建模型
    model_cfg = config['model']
    model = create_model(
        architecture=model_cfg['architecture'],
        encoder_name=model_cfg['encoder_name'],
        encoder_weights=None,  # 不使用预训练权重加速
        in_channels=model_cfg['in_channels'],
        classes=model_cfg['out_channels'],
        activation=model_cfg['activation']
    ).to(device)
    
    print(f"模型: {model_cfg['architecture']} + {model_cfg['encoder_name']}")
    
    # 加载已有模型权重（如果存在）
    checkpoint_path = Path('checkpoints/best_model.pth')
    if checkpoint_path.exists():
        print(f"加载已有模型: {checkpoint_path}")
        model.load_state_dict(torch.load(checkpoint_path, map_location=device))
    
    # 创建数据集（只使用部分数据）
    print("\n创建数据集...")
    print("权重配置: 38cloud=1.0, 95cloud=2.0")
    
    # 创建单个数据集
    ds_38 = CloudSegmentationDataset("38cloud", skip_invalid=True)
    ds_95 = CloudSegmentationDataset("95cloud", skip_invalid=True)
    
    print(f"38-Cloud: {len(ds_38)} 样本")
    print(f"95-Cloud: {len(ds_95)} 样本")
    
    # 只使用部分数据（每数据集1000个样本）
    max_samples = 1000
    indices_38 = np.random.choice(len(ds_38), min(max_samples, len(ds_38)), replace=False)
    indices_95 = np.random.choice(len(ds_95), min(max_samples, len(ds_95)), replace=False)
    
    ds_38_subset = Subset(ds_38, indices_38)
    ds_95_subset = Subset(ds_95, indices_95)
    
    print(f"使用子集: 38cloud={len(ds_38_subset)}, 95cloud={len(ds_95_subset)}")
    
    # 合并数据集
    from torch.utils.data import ConcatDataset
    combined_dataset = ConcatDataset([ds_38_subset, ds_95_subset])
    
    # 创建 DataLoader
    train_loader = torch.utils.data.DataLoader(
        combined_dataset,
        batch_size=config['training']['batch_size'],
        shuffle=True,
        num_workers=0
    )
    
    print(f"总训练样本: {len(combined_dataset)}")
    print(f"批次数: {len(train_loader)}")
    
    # 损失函数和优化器
    criterion = BCEWithDiceLoss()
    optimizer = torch.optim.Adam(
        model.parameters(),
        lr=config['training']['learning_rate'],
        weight_decay=config['training']['weight_decay']
    )
    
    # 训练循环（只训练5轮）
    num_epochs = 5
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
        print(f"Epoch {epoch+1}/{num_epochs} - 平均损失: {avg_loss:.4f}")
        
        # 保存最佳模型
        if avg_loss < best_loss:
            best_loss = avg_loss
            save_path = Path('checkpoints')
            save_path.mkdir(exist_ok=True)
            torch.save(model.state_dict(), save_path / 'best_model_weighted.pth')
            print(f"  ✓ 保存最佳模型 (损失: {best_loss:.4f})")
    
    print("\n" + "=" * 60)
    print("训练完成!")
    print(f"最佳损失: {best_loss:.4f}")
    print(f"模型保存于: checkpoints/best_model_weighted.pth")
    print("=" * 60)
    
    return model


if __name__ == "__main__":
    train_weighted_quick()
