
"""
快速验证模型脚本 - 测试模型架构是否能正常构建和运行
"""
import torch
from models.unet import UNet, UNetWithBackbone

def verify_unet():
    """验证原始 UNet 模型"""
    print("=" * 60)
    print("验证 UNet 模型...")
    print("=" * 60)
    
    model = UNet(in_channels=3, out_channels=1, init_features=64)
    total_params = sum(p.numel() for p in model.parameters())
    print(f"✓ UNet 模型创建成功，参数量: {total_params:,}")
    
    # 测试前向传播
    test_input = torch.randn(1, 3, 256, 256)
    with torch.no_grad():
        output = model(test_input)
    print(f"✓ 前向传播成功，输入形状: {test_input.shape}，输出形状: {output.shape}")
    
    assert output.shape == (1, 1, 256, 256), f"输出形状不正确，期望 (1, 1, 256, 256)，得到 {output.shape}"
    print("✓ 输出形状正确")
    
    return True

def verify_unet_with_backbone(backbone_name):
    """验证带骨干网络的 UNet 模型"""
    print("\n" + "=" * 60)
    print(f"验证 UNetWithBackbone (骨干网络: {backbone_name})...")
    print("=" * 60)
    
    try:
        model = UNetWithBackbone(backbone_name=backbone_name, num_classes=1, pretrained=False)
        total_params = sum(p.numel() for p in model.parameters())
        print(f"✓ UNetWithBackbone({backbone_name}) 模型创建成功，参数量: {total_params:,}")
        
        # 测试前向传播
        test_input = torch.randn(1, 3, 256, 256)
        with torch.no_grad():
            output = model(test_input)
        print(f"✓ 前向传播成功，输入形状: {test_input.shape}，输出形状: {output.shape}")
        
        assert output.shape == (1, 1, 256, 256), f"输出形状不正确，期望 (1, 1, 256, 256)，得到 {output.shape}"
        print("✓ 输出形状正确")
        
        return True
    except Exception as e:
        print(f"✗ 验证失败: {e}")
        return False

def main():
    print("\n" + "#" * 60)
    print("# 云分割模型验证")
    print("#" * 60 + "\n")
    
    results = {}
    
    # 验证原始 UNet
    results["UNet"] = verify_unet()
    
    # 验证各种骨干网络
    backbones = ["resnet18", "resnet34", "resnet50", "resnet101", "efficientnet_b4", "mobilenet_v2"]
    
    for backbone in backbones:
        results[f"UNetWithBackbone({backbone})"] = verify_unet_with_backbone(backbone)
    
    # 打印总结
    print("\n" + "#" * 60)
    print("# 验证总结")
    print("#" * 60)
    for name, success in results.items():
        status = "✓ 通过" if success else "✗ 失败"
        print(f"{name:40s} {status}")
    
    print("\n" + "=" * 60)
    all_passed = all(results.values())
    if all_passed:
        print("✓ 所有模型验证通过！")
    else:
        print("✗ 部分模型验证失败")
    print("=" * 60)

if __name__ == "__main__":
    main()

