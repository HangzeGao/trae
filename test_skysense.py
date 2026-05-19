#!/usr/bin/env python3
"""
SkySense++ Model Test Script
验证模型架构是否正确工作
"""

import torch
import torch.nn as nn
import sys
from pathlib import Path

project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from models.skysense import SkySensePlusPlus, create_skysense_model


def test_model_forward():
    """测试模型前向传播"""
    print("测试 SkySense++ 模型前向传播...")
    
    model = SkySensePlusPlus(
        in_channels=4,
        embed_dim=128,
        num_heads=4,
        num_layers=3,
        patch_size=16,
        out_channels=1
    )
    
    model.eval()
    
    # 测试不同输入大小
    test_sizes = [(256, 256), (512, 512), (128, 128)]
    
    for H, W in test_sizes:
        x = torch.randn(2, 4, H, W)
        
        with torch.no_grad():
            y = model(x)
        
        print(f"  输入: {x.shape}, 输出: {y.shape}")
        assert y.shape == (2, 1, H, W), f"输出形状不匹配: {y.shape}"
    
    print("  ✓ 前向传播测试通过")


def test_model_parameters():
    """测试模型参数数量"""
    print("\n测试模型参数数量...")
    
    model = SkySensePlusPlus(
        in_channels=4,
        embed_dim=128,
        num_heads=4,
        num_layers=3,
        patch_size=16
    )
    
    total_params = sum(p.numel() for p in model.parameters())
    trainable_params = sum(p.numel() for p in model.parameters() if p.requires_grad)
    
    print(f"  总参数: {total_params:,}")
    print(f"  可训练参数: {trainable_params:,}")
    
    assert total_params == trainable_params, "存在不可训练参数"
    print("  ✓ 参数测试通过")


def test_model_training():
    """测试模型训练过程"""
    print("\n测试模型训练过程...")
    
    model = SkySensePlusPlus(
        in_channels=4,
        embed_dim=128,
        num_heads=4,
        num_layers=3,
        patch_size=16
    )
    
    optimizer = torch.optim.Adam(model.parameters(), lr=0.001)
    criterion = nn.BCELoss()
    
    model.train()
    
    x = torch.randn(2, 4, 256, 256)
    y_true = torch.rand(2, 1, 256, 256)
    
    for i in range(3):
        optimizer.zero_grad()
        
        y_pred = model(x)
        loss = criterion(y_pred, y_true)
        
        loss.backward()
        optimizer.step()
        
        print(f"  迭代 {i+1}: loss = {loss.item():.6f}")
    
    print("  ✓ 训练测试通过")


def test_create_model_from_config():
    """测试从配置创建模型"""
    print("\n测试从配置创建模型...")
    
    class MockConfig:
        model = {
            'in_channels': 4,
            'embed_dim': 64,
            'num_heads': 2,
            'num_layers': 2,
            'patch_size': 16,
            'out_channels': 1
        }
    
    model = create_skysense_model(MockConfig())
    
    x = torch.randn(1, 4, 256, 256)
    y = model(x)
    
    print(f"  输入: {x.shape}, 输出: {y.shape}")
    assert y.shape == (1, 1, 256, 256)
    print("  ✓ 配置创建测试通过")


def test_model_components():
    """测试各个组件"""
    print("\n测试模型组件...")
    
    from models.skysense import VisionTransformer, SpatioTemporalEncoder, MultiGranularityFusion, SegmentationHead
    
    # Test VisionTransformer
    vit = VisionTransformer(in_channels=4, embed_dim=128, num_heads=4, num_layers=2, patch_size=16)
    x = torch.randn(2, 4, 256, 256)
    cls_token, patch_tokens = vit(x)
    print(f"  VisionTransformer: cls_token={cls_token.shape}, patch_tokens={patch_tokens.shape}")
    
    # Test SpatioTemporalEncoder
    encoder = SpatioTemporalEncoder(in_channels=4, embed_dim=128, num_heads=4, num_layers=2, patch_size=16)
    x = torch.randn(2, 4, 256, 256)
    global_feat, local_feat = encoder(x)
    print(f"  SpatioTemporalEncoder: global={global_feat.shape}, local={local_feat.shape}")
    
    # Test MultiGranularityFusion
    fusion = MultiGranularityFusion(embed_dim=128, num_heads=4)
    fused = fusion(local_feat, global_feat, 256, 256, 16)
    print(f"  MultiGranularityFusion: fused={fused.shape}")
    
    # Test SegmentationHead
    head = SegmentationHead(in_channels=128, out_channels=1)
    out = head(fused)
    print(f"  SegmentationHead: out={out.shape}")
    
    print("  ✓ 组件测试通过")


def main():
    print("=" * 80)
    print("SkySense++ 模型测试")
    print("=" * 80)
    
    test_model_forward()
    test_model_parameters()
    test_model_training()
    test_create_model_from_config()
    test_model_components()
    
    print("\n" + "=" * 80)
    print("所有测试通过!")
    print("=" * 80)


if __name__ == "__main__":
    main()