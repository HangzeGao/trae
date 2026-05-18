"""
快速验证模型脚本
测试 segmentation_models_pytorch 的基本功能
"""
import torch
import sys

print("=" * 60)
print("模型验证测试")
print("=" * 60)

# 测试1: 检查 PyTorch
print("\n[测试 1] 检查 PyTorch...")
try:
    print(f"  PyTorch 版本: {torch.__version__}")
    print(f"  CUDA 可用: {torch.cuda.is_available()}")
    if torch.cuda.is_available():
        print(f"  CUDA 版本: {torch.version.cuda}")
        print(f"  GPU 设备: {torch.cuda.get_device_name(0)}")
    print("  ✓ PyTorch 检查通过")
except Exception as e:
    print(f"  ✗ PyTorch 检查失败: {e}")
    sys.exit(1)

# 测试2: 检查 segmentation_models_pytorch
print("\n[测试 2] 检查 segmentation_models_pytorch...")
try:
    import segmentation_models_pytorch as smp
    print(f"  SMP 版本: {smp.__version__}")
    print("  ✓ segmentation_models_pytorch 检查通过")
except ImportError as e:
    print(f"  ✗ segmentation_models_pytorch 未安装")
    print(f"  错误: {e}")
    print("\n  请运行以下命令安装:")
    print("  pip install segmentation-models-pytorch")
    sys.exit(1)

# 测试3: 导入项目模块
print("\n[测试 3] 导入项目模型模块...")
try:
    from models.unet import create_model
    print("  ✓ 项目模块导入成功")
except Exception as e:
    print(f"  ✗ 项目模块导入失败: {e}")
    sys.exit(1)

# 测试4: 创建不同架构的模型
print("\n[测试 4] 测试不同架构模型创建...")
architectures = ['Unet', 'DeepLabV3Plus', 'FPN', 'Linknet']
encoder = 'resnet34'

for arch in architectures:
    try:
        print(f"\n  测试 {arch}...")
        model = create_model(
            architecture=arch,
            encoder_name=encoder,
            encoder_weights=None,
            in_channels=3,
            classes=1,
            activation='sigmoid'
        )
        
        # 设置为评估模式以避免 BatchNorm 问题
        model.eval()
        
        # 测试前向传播
        x = torch.randn(1, 3, 256, 256)
        with torch.no_grad():
            y = model(x)
        
        params = sum(p.numel() for p in model.parameters())
        print(f"    输入形状: {x.shape}")
        print(f"    输出形状: {y.shape}")
        print(f"    参数量: {params:,}")
        print(f"    ✓ {arch} 测试通过")
        
    except Exception as e:
        print(f"    ✗ {arch} 测试失败: {e}")
        import traceback
        traceback.print_exc()

# 测试5: 测试预训练权重加载
print("\n[测试 5] 测试预训练权重...")
try:
    print("  创建带预训练权重的模型...")
    model_pretrained = create_model(
        architecture='Unet',
        encoder_name='resnet34',
        encoder_weights='imagenet',
        in_channels=3,
        classes=1,
        activation='sigmoid'
    )
    
    model_pretrained.eval()
    params = sum(p.numel() for p in model_pretrained.parameters())
    print(f"  参数量: {params:,}")
    
    # 测试前向传播
    x = torch.randn(1, 3, 256, 256)
    with torch.no_grad():
        y = model_pretrained(x)
    print(f"  输出形状: {y.shape}")
    print("  ✓ 预训练模型创建成功")
except Exception as e:
    print(f"  ✗ 预训练模型创建失败: {e}")
    print("  (这可能是因为网络问题下载预训练权重失败)")

# 测试6: 测试不同编码器
print("\n[测试 6] 测试不同编码器...")
encoders = ['resnet18', 'mobilenet_v2', 'efficientnet-b0']
arch = 'Unet'

for enc in encoders:
    try:
        print(f"\n  测试 {enc}...")
        model = create_model(
            architecture=arch,
            encoder_name=enc,
            encoder_weights=None,
            in_channels=3,
            classes=1,
            activation='sigmoid'
        )
        
        model.eval()
        params = sum(p.numel() for p in model.parameters())
        print(f"    参数量: {params:,}")
        print(f"    ✓ {enc} 测试通过")
        
    except Exception as e:
        print(f"    ✗ {enc} 测试失败: {e}")

# 测试7: 测试 Unet++ 和其他架构
print("\n[测试 7] 测试其他架构...")
architectures_extra = ['UnetPlusPlus', 'PSPNet']
for arch in architectures_extra:
    try:
        print(f"\n  测试 {arch}...")
        model = create_model(
            architecture=arch,
            encoder_name='resnet34',
            encoder_weights=None,
            in_channels=3,
            classes=1,
            activation='sigmoid'
        )
        
        model.eval()
        x = torch.randn(1, 3, 256, 256)
        with torch.no_grad():
            y = model(x)
        
        params = sum(p.numel() for p in model.parameters())
        print(f"    参数量: {params:,}")
        print(f"    输出形状: {y.shape}")
        print(f"    ✓ {arch} 测试通过")
        
    except Exception as e:
        print(f"    ✗ {arch} 测试失败: {e}")

# 总结
print("\n" + "=" * 60)
print("✓ 所有验证测试完成!")
print("=" * 60)

print("\n模型使用示例:")
print("  # 在 Python 中使用")
print("  from models.unet import create_model")
print("  ")
print("  # 创建 Unet 模型")
print("  model = create_model(")
print("      architecture='Unet',")
print("      encoder_name='resnet34',")
print("      encoder_weights='imagenet',")
print("      in_channels=3,")
print("      classes=1,")
print("      activation='sigmoid'")
print("  )")
print("  ")
print("  # 或者在配置文件中设置:")
print("  # model:")
print("  #   architecture: 'Unet'")
print("  #   encoder_name: 'resnet34'")
print("  #   encoder_weights: 'imagenet'")
