"""
使用快速验证采样策略训练（Sample Strategy）
从每个数据集采样少量样本进行快速训练和验证
"""
import yaml
import torch
import numpy as np
from pathlib import Path
from tqdm import tqdm

from models.unet import create_model
from models.losses import BCEWithDiceLoss
from models.metrics import SegmentationMetrics
from data.multi_dataset_loader import get_combined_dataloader


def train_sample(strategy="sample", sample_size=50):
    """使用快速验证采样策略训练"""
    
    # 加载配置
    with open('config/config.yaml') as f:
        config = yaml.safe_load(f)
    
    print("=" * 60)
    print(f"快速验证采样训练 (策略: {strategy}, 采样数: {sample_size})")
    print("=" * 60)
    
    # 设备
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"设备: {device}")
    
    # 创建模型
    model_cfg = config['model']
    model = create_model(
        architecture=model_cfg['architecture'],
        encoder_name=model_cfg['encoder_name'],
        encoder_weights=None,
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
    
    # 创建 DataLoader（使用快速验证采样策略）
    print("\n创建 DataLoader...")
    train_loader = get_combined_dataloader(
        dataset_names=["38cloud", "95cloud"],
        strategy=strategy,
        batch_size=config['training']['batch_size'],
        shuffle=True,
        num_workers=0,
        sample_size=sample_size,
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
    
    # 训练循环（只训练3轮）
    num_epochs = 3
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
            
            pbar.set_postfix({
                'loss': f'{loss.item():.4f}',
                'avg_loss': f'{epoch_loss/(batch_idx+1):.4f}'
            })
        
        avg_loss = epoch_loss / len(train_loader)
        print(f"Epoch {epoch+1}/{num_epochs} - 平均损失: {avg_loss:.4f}")
        
        # 保存模型
        if avg_loss < best_loss:
            best_loss = avg_loss
            save_path = Path('checkpoints')
            save_path.mkdir(exist_ok=True)
            torch.save(model.state_dict(), save_path / 'best_model_sample.pth')
            print(f"  ✓ 保存模型 (损失: {best_loss:.4f})")
    
    # 快速验证
    print("\n快速验证...")
    model.eval()
    
    val_dice = []
    val_iou = []
    
    with torch.no_grad():
        for images, masks in train_loader:
            images = images.to(device)
            masks = masks.to(device)
            
            outputs = model(images)
            
            # 计算指标
            dice = SegmentationMetrics.dice_score(outputs, masks)
            iou = SegmentationMetrics.iou_score(outputs, masks)
            
            val_dice.append(dice)
            val_iou.append(iou)
    
    avg_dice = np.mean(val_dice)
    avg_iou = np.mean(val_iou)
    
    print(f"\n验证结果:")
    print(f"平均 Dice Score: {avg_dice:.4f}")
    print(f"平均 IoU Score:  {avg_iou:.4f}")
    
    print("\n" + "=" * 60)
    print("快速训练完成!")
    print(f"模型保存于: checkpoints/best_model_sample.pth")
    print("=" * 60)
    
    return model, avg_dice, avg_iou


if __name__ == "__main__":
    train_sample(strategy="sample", sample_size=50)
