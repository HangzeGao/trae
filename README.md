# Cloud Segmentation Project

## 项目概述

这是一个基于深度学习的卫星图像云分割项目，使用 PyTorch 和 segmentation-models-pytorch 实现。

## 支持的数据集

### 1. 38-Cloud Dataset
- **来源**: Kaggle - sorour/38cloud-cloud-segmentation-in-satellite-images
- **训练样本**: 8,400 张
- **测试样本**: 9,201 张
- **波段**: 红、绿、蓝、近红外 (RGB + NIR)
- **文件格式**: GeoTIFF (.TIF)

### 2. 95-Cloud Dataset  
- **来源**: Kaggle - sorour/95cloud-cloud-segmentation-on-satellite-images
- **训练样本**: 34,701 张
- **波段**: 红、绿、蓝、近红外 (RGB + NIR)
- **文件格式**: GeoTIFF (.TIF)

### 3. Cloud Cover Detection Dataset
- **来源**: Kaggle - hmendonca/cloud-cover-detection
- **状态**: 待下载
- **特性**: 多样化的云检测标注

## 项目结构

```
cloud-segmentation/
├── config/
│   └── config.yaml              # 配置文件
├── data/
│   ├── 38cloud/                 # 38-Cloud 数据集调度文件
│   ├── 95cloud/                 # 95-Cloud 数据集调度文件
│   ├── cloud-cover/             # Cloud Cover Detection 调度文件
│   ├── kaggle_dataset_loader.py # 统一的数据加载器
│   ├── process_new_dataset.py   # 处理新数据集的脚本
│   └── master_schedule.csv      # 合并的调度文件
├── models/
│   ├── unet.py                  # UNet 模型 (基于 SMP)
│   ├── losses.py                # 损失函数
│   └── metrics.py               # 评估指标
├── checkpoints/                 # 保存的模型
├── train_cloud_segmentation.py  # 训练脚本
├── test_best_model.py           # 测试脚本
├── requirements.txt             # 依赖
└── README.md                    # 本文档
```

## 快速开始

### 1. 安装依赖

```bash
pip install torch torchvision segmentation-models-pytorch \
  pyyaml pandas pillow matplotlib tqdm
```

### 2. 下载数据集

```python
import kagglehub

# 38-Cloud
path1 = kagglehub.dataset_download("sorour/38cloud-cloud-segmentation-in-satellite-images")

# 95-Cloud
path2 = kagglehub.dataset_download("sorour/95cloud-cloud-segmentation-on-satellite-images")

# Cloud Cover Detection
path3 = kagglehub.dataset_download("hmendonca/cloud-cover-detection")
```

### 3. 使用统一的数据加载器

```python
from data.kaggle_dataset_loader import (
    CloudSegmentationDataset,
    create_dataloader,
    create_combined_dataloader,
    list_available_datasets
)

# 列出所有数据集
print(list_available_datasets())

# 创建单个数据集的 DataLoader
train_loader = create_dataloader(
    dataset_name='38cloud',
    batch_size=8,
    shuffle=True
)

# 合并多个数据集
combined_loader = create_combined_dataloader(
    dataset_names=['38cloud', '95cloud'],
    batch_size=8
)

# 使用自定义数据集
dataset = CloudSegmentationDataset(
    dataset_name='38cloud',
    schedule_path='/path/to/your/schedule.csv'
)
```

### 4. 训练模型

```bash
python train_cloud_segmentation.py
```

### 5. 测试模型

```bash
python test_best_model.py
```

## 模型架构

### 当前配置
- **分割模型**: UNet
- **编码器**: ResNet-34 (ImageNet 预训练)
- **输入通道**: 4 (RGB + NIR)
- **输出通道**: 1 (二分类: 云/非云)
- **损失函数**: Dice + BCE 组合损失

### 可选的架构和编码器

支持的分割架构:
- UNet, UNet++, DeepLabV3, DeepLabV3Plus, FPN, PSPNet, Linknet, MANet, PAN

支持的编码器:
- ResNet 系列: resnet18, resnet34, resnet50, resnet101
- EfficientNet: efficientnet-b0 到 efficientnet-b7
- MobileNet: mobilenet_v2
- 其他: inception_v4, densenet121, 等

## 训练结果

### 38-Cloud 数据集
- **训练样本**: 237 个 (有效样本，云覆盖率 10%-90%)
- **验证样本**: 60 个
- **最佳 Dice Score**: 0.6109
- **最佳 IoU Score**: 0.4452
- **训练轮数**: 16 epochs (早停)
- **测试 Dice Score**: 0.9239 (精选测试样本)

## 配置说明

配置文件位于 `config/config.yaml`:

```yaml
# 模型配置
model:
  architecture: "Unet"
  encoder_name: "resnet34"
  encoder_weights: "imagenet"
  in_channels: 4
  out_channels: 1

# 训练配置
training:
  epochs: 20
  batch_size: 8
  learning_rate: 0.001
  weight_decay: 0.00001
  optimizer: "adam"
  loss: "dice_bce"
  patience: 8

# 数据配置
data:
  min_cloud_ratio: 0.1   # 最小云覆盖率
  max_cloud_ratio: 0.9   # 最大云覆盖率
  train_val_split: 0.8   # 训练/验证集划分比例
```

## API 参考

### 数据加载器 API

#### `CloudSegmentationDataset`
统一的云分割数据集类。

**参数:**
- `dataset_name`: 数据集名称 ('38cloud', '95cloud', 'cloud-cover')
- `schedule_path`: 调度文件路径 (可选)
- `transform`: 数据变换 (可选)
- `normalize`: 是否归一化 (默认: True)
- `mask_threshold`: 掩码二值化阈值 (可选)

#### `create_dataloader()`
创建单个数据集的 DataLoader。

#### `create_combined_dataloader()`
创建合并多个数据集的 DataLoader。

#### `list_available_datasets()`
列出所有可用的数据集。

#### `get_dataset_info(dataset_name)`
获取指定数据集的详细信息。

## 许可证

本项目仅供学习和研究使用。

## 参考资料

- [Segmentation Models PyTorch](https://github.com/qubvel/segmentation_models.pytorch)
- [38-Cloud Dataset](https://www.kaggle.com/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images)
- [95-Cloud Dataset](https://www.kaggle.com/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images)
- [Cloud Cover Detection Dataset](https://www.kaggle.com/datasets/hmendonca/cloud-cover-detection)
