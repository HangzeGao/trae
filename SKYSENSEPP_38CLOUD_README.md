# SkySense++ 38Cloud 训练和验证

## 概述

本项目实现了 **SkySense++** 模型在 **38Cloud** 数据集上的训练、验证和可视化。

- 模型: SkySense++ (语义增强的多模态遥感基础模型)
- 数据集: 38Cloud (云分割数据集，4通道: blue, green, red, NIR)
- 任务: 云/背景二分类分割

## 文件说明

| 文件 | 说明 |
|------|------|
| [models/skysense_pp.py](models/skysense_pp.py) | SkySense++ 模型实现 |
| [run_skysensepp_38cloud.py](run_skysensepp_38cloud.py) | 完整训练和验证脚本 (推荐使用) |
| [train_skysensepp_38cloud.py](train_skysensepp_38cloud.py) | 集成现有数据加载器的训练脚本 |
| [setup_and_run.sh](setup_and_run.sh) | 安装依赖和运行脚本的一键脚本 |
| [config/config.yaml](config/config.yaml) | 配置文件 (已更新支持SkySense++) |

## SkySense++ 模型特点

### 核心组件

1. **多模态融合**
   - 自适应融合 (`adaptive`): 学习性的特征融合
   - Transformer融合: 基于注意力的跨模态融合
   - 拼接融合: 简单高效的基础方法

2. **语义增强模块**
   - 通道注意力机制
   - 空间注意力机制
   - 上下文聚合

3. **架构参数**
   - 编码器: ResNet-34 (预训练支持)
   - 解码器: 渐进式上采样 + 跳跃连接
   - 激活函数: Sigmoid (二分类)

## 快速开始

### 方法1: 一键运行 (推荐)

```bash
# 设置执行权限
chmod +x setup_and_run.sh

# 运行脚本 (安装依赖 + 训练 + 验证 + 可视化)
./setup_and_run.sh
```

### 方法2: 分步运行

```bash
# 1. 安装依赖
pip install torch torchvision
pip install segmentation-models-pytorch
pip install kagglehub numpy pillow matplotlib tqdm pandas pyyaml

# 2. 运行训练和验证
python run_skysensepp_38cloud.py
```

## 运行流程

脚本自动执行以下步骤:

1. **下载数据集**
   - 使用 kagglehub 下载 38Cloud 数据集
   - 或使用已有缓存

2. **创建数据加载器**
   - 80% 训练，20% 验证
   - 加载 4通道影像 (blue, green, red, NIR)
   - 加载云掩码 (GT)

3. **创建 SkySense++ 模型**
   - 输入: 4通道
   - 输出: 1通道二值分割结果

4. **训练模型**
   - 损失函数: BCELoss
   - 优化器: Adam (lr=0.001)
   - 轮数: 5轮 (演示，可修改)
   - 保存最佳模型

5. **验证和可视化**
   - 计算 Dice 系数
   - 生成预测结果可视化
   - 绘制训练曲线

## 输出结果

训练完成后，结果将保存在:

```
38cloud_results/
├── sample_001.png      # 可视化结果 1
├── sample_002.png      # 可视化结果 2
├── ...
└── training_curves.png # 训练曲线
```

每个可视化结果包含:
- **输入 (RGB)**: 前三个波段的彩色图像
- **近红外 (NIR)**: 第4通道的灰度图
- **真实掩码**: 数据集标注的云区域
- **预测结果**: SkySense++的预测结果

## 模型配置

在代码中可以修改以下参数:

```python
model = SkySensePPModel(
    encoder_name='resnet34',        # 编码器: resnet18/34/50
    in_channels=4,                   # 输入通道: 4 (RGB+NIR)
    num_classes=1,                   # 输出通道: 1 (二分类)
    fusion_type='adaptive',          # 融合方式: adaptive/transformer/concat
    use_semantic_enhancement=True,   # 是否使用语义增强
    dropout=0.1,                     # Dropout比率
    activation='sigmoid'             # 激活函数
)
```

## 训练配置

在 `run_skysensepp_38cloud.py` 中可以修改:

- `num_epochs = 5`: 训练轮数 (可增加到20-50)
- `batch_size = 4`: 批次大小
- `lr = 0.001`: 学习率

## 原始 SkySense++ 论文

如果你对完整的 SkySense++ 感兴趣，请参考:

- **论文**: [Nature Machine Intelligence 2025](https://www.nature.com/articles/s42256-025-01078-8)
- **代码**: [GitHub - SkySensePlusPlus](https://github.com/kang-wu/SkySensePlusPlus)
- **特点**: 支持多模态 (RGB/NIR/SAR) 多时相输入，语义增强预训练

## 项目结构

```
/workspace/
├── models/
│   ├── skysense_pp.py      # SkySense++ 模型
│   ├── unet.py             # 其他分割模型
│   └── __init__.py
├── data/
│   └── multi_dataset_loader.py  # 数据加载
├── config/
│   └── config.yaml         # 配置文件
├── run_skysensepp_38cloud.py  # 完整训练脚本
├── setup_and_run.sh        # 一键运行
└── 38cloud_results/        # 结果目录 (运行后生成)
```

## 常见问题

### Q: 找不到数据集怎么办？

确保:
1. kagglehub 已正确安装
2. 网络连接正常
3. 或手动下载并放到指定路径

### Q: 显存不足怎么办？

减小 batch_size:
```python
train_loader = DataLoader(train_dataset, batch_size=2, shuffle=True)
```

### Q: 如何训练更长时间？

修改:
```python
num_epochs = 20  # 或更多
```

## 许可证

本项目仅供学习和研究使用。

SkySense++ 的预训练权重和原始代码遵循原作者许可证要求。
