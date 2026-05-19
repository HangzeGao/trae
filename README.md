# Cloud Segmentation Project

基于深度学习的卫星图像云分割项目，支持多数据集训练和验证。

## 特性

- 🏗️ **模块化设计**：模型、数据、配置分离，易于扩展
- 🎯 **多数据集支持**：支持 38-Cloud、95-Cloud 等数据集的合并训练
- ⚡ **多种采样策略**：加权采样、轮询采样、平衡采样等
- 🍎 **跨平台支持**：支持 CUDA (NVIDIA) 和 MPS (Apple M 芯片)
- 📊 **统一配置管理**：所有配置集中管理，易于调整

## 项目结构

```
cloud-segmentation/
├── config/
│   └── config.yaml              # 配置文件（数据集路径、模型参数、训练策略）
├── data/
│   └── multi_dataset_loader.py   # 多数据集加载器
├── models/
│   ├── __init__.py
│   ├── unet.py                  # 分割模型
│   ├── losses.py                # 损失函数
│   └── metrics.py               # 评估指标
├── utils/
│   ├── __init__.py
│   ├── config.py                # 配置加载器
│   └── helpers.py               # 工具函数（Apple M 支持）
├── train.py                     # 训练脚本
├── validate.py                  # 验证脚本
├── requirements.txt
└── README.md
```

## 快速开始

### 1. 安装依赖

```bash
pip install -r requirements.txt
```

### 2. 配置数据集

编辑 `config/config.yaml`，配置数据集路径：

```yaml
datasets:
  "38cloud":
    base_path: "/path/to/38cloud"
    schedule_file: "./data/38cloud/train_patches.csv"
    bands: ["blue", "green", "red", "nir"]
    weight: 1.0
  "95cloud":
    base_path: "/path/to/95cloud"
    schedule_file: "./data/95cloud/train_patches.csv"
    bands: ["blue", "green", "red", "nir"]
    weight: 2.0
```

### 3. 下载数据集

```python
import kagglehub

# 38-Cloud
path1 = kagglehub.dataset_download("sorour/38cloud-cloud-segmentation-in-satellite-images")

# 95-Cloud
path2 = kagglehub.dataset_download("sorour/95cloud-cloud-segmentation-on-satellite-images")
```

### 4. 训练模型

```bash
# 默认：加权采样策略
python train.py

# 指定数据集
python train.py --datasets 38cloud 95cloud

# 快速验证（采样模式）
python train.py --strategy sample --quick

# 轮询采样策略
python train.py --strategy round_robin

# 平衡采样策略
python train.py --strategy balanced
```

### 5. 验证模型

```bash
python validate.py --checkpoint checkpoints/best_model_weighted.pth

# 指定数据集
python validate.py --checkpoint checkpoints/best_model.pth --datasets 38cloud
```

## 配置说明

所有配置位于 `config/config.yaml`：

```yaml
# 模型配置
model:
  architecture: "Unet"           # UNet, UNet++, DeepLabV3, DeepLabV3Plus, FPN
  encoder_name: "resnet34"        # resnet18, resnet50, efficientnet-b0, mobilenet_v2
  encoder_weights: "imagenet"
  in_channels: 4                 # RGB + NIR
  out_channels: 1

# 训练配置
training:
  epochs: 20
  batch_size: 8
  learning_rate: 0.001
  loss: "dice_bce"               # dice, bce, focal, iou
  checkpoint_dir: "./checkpoints"

# 采样策略
sampling:
  default_strategy: "weighted"    # concat, weighted, round_robin, balanced, sample
  sample_size: 100               # 快速验证时每数据集采样数量
```

## 采样策略

| 策略 | 说明 | 使用场景 |
|------|------|---------|
| `concat` | 简单拼接所有数据集 | 训练 |
| `weighted` | 根据权重随机采样 | 推荐训练 |
| `round_robin` | 轮询从每个数据集取样 | 训练 |
| `balanced` | 每个数据集取相同数量 | 训练/验证 |
| `sample` | 从每个数据集采样少量 | 快速验证 |

## 支持的设备

自动检测可用设备：

1. **CUDA** - NVIDIA GPU
2. **MPS** - Apple M 芯片 (Metal Performance Shaders)
3. **CPU** - 后备方案

## API 使用

### 创建数据加载器

```python
from data.multi_dataset_loader import get_combined_dataloader, print_dataset_summary

# 打印可用数据集
print_dataset_summary()

# 创建合并数据加载器
loader = get_combined_dataloader(
    dataset_names=["38cloud", "95cloud"],
    strategy="weighted",
    batch_size=8,
    config=config
)
```

### 创建模型

```python
from models.unet import create_model_from_config
from models.losses import get_loss_function_from_config

model = create_model_from_config(config)
criterion = get_loss_function_from_config(config)
```

## 模型架构

支持的分割模型：

- **UNet** - 经典编码器-解码器结构
- **UNet++** - 嵌套跳跃连接
- **DeepLabV3** - 空洞空间金字塔池化
- **DeepLabV3Plus** - 增强版 DeepLabV3
- **FPN** - 特征金字塔网络
- **PSPNet** - 金字塔场景解析

支持的编码器：

- ResNet 系列: resnet18, resnet34, resnet50
- EfficientNet: efficientnet-b0 到 b7
- MobileNet: mobilenet_v2
- 其他: densenet121, inception_v4 等

## 许可证

本项目仅供学习和研究使用。

## 参考资料

- [Segmentation Models PyTorch](https://github.com/qubvel/segmentation_models.pytorch)
- [38-Cloud Dataset](https://www.kaggle.com/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images)
- [95-Cloud Dataset](https://www.kaggle.com/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images)
