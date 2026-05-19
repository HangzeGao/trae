# SkySense++ Model Integration Guide

## Overview

This document describes the integration of **SkySense++** architecture into the existing cloud segmentation project. SkySense++ is a semantic-enhanced multi-modal remote sensing foundation model published in Nature Machine Intelligence (2025).

## Reference

- **Original Repository**: [SkySense++](https://github.com/kang-wu/SkySensePlusPlus)
- **Paper**: [SkySense++: A Semantic-Enhanced Multi-Modal Remote Sensing Foundation Model Beyond SkySense for Earth Observation](https://www.nature.com/articles/s42256-025-01078-8)

## Key Features

### 1. Multi-Modal Fusion
- **Adaptive Fusion**: Learnable fusion of multi-modal features
- **Transformer Fusion**: Transformer-based feature fusion
- Supports RGB, NIR, SAR, and Sentinel-2 time series data

### 2. Semantic Enhancement
- Channel-wise attention mechanism
- Spatial attention for focusing on important regions
- Context aggregation for semantic understanding

### 3. Architecture Components

#### ConvModule
- Convolution with batch normalization
- Configurable activation functions (ReLU, GELU, SiLU)

#### MultiModalFusion
- Adaptive fusion of multi-modal features
- Transformer-based fusion option
- Automatic channel alignment

#### SemanticEnhancementModule
- Channel attention
- Spatial attention
- Combined semantic enhancement

#### SemanticEnhancedDecoder
- Skip connections from encoder
- Hierarchical feature decoding
- Multi-scale feature aggregation

## Usage

### Configuration

Add SkySense++ to your `config/config.yaml`:

```yaml
model:
  architecture: "SkySensePP"
  encoder_name: "resnet34"
  encoder_weights: "imagenet"
  in_channels: 4
  out_channels: 1
  activation: "sigmoid"
  
  # SkySense++ specific
  fusion_type: "adaptive"  # adaptive, transformer, concat
  use_semantic_enhancement: true
  encoder_depth: 5
  decoder_channels: [256, 128, 64, 32, 16]
  dropout: 0.1
```

### Code Usage

#### Direct Model Creation

```python
from models.skysense_pp import SkySensePPModel

model = SkySensePPModel(
    encoder_name='resnet34',
    in_channels=4,
    num_classes=1,
    fusion_type='adaptive',
    use_semantic_enhancement=True
)

# Forward pass
x = torch.randn(1, 4, 256, 256)
output = model(x)  # Shape: [1, 1, 256, 256]
```

#### Using Config

```python
from utils.config import Config
from models import create_model_from_config

config = Config('config/config.yaml')
model = create_model_from_config(config)

output = model(input_tensor)
```

### Training

The model can be used with existing training scripts:

```bash
python train.py --config config/config.yaml
```

### Fusion Types

#### 1. Adaptive Fusion (Default)
```python
fusion_type='adaptive'
```
- Learns attention weights for feature fusion
- Balanced complexity and performance
- Suitable for most scenarios

#### 2. Transformer Fusion
```python
fusion_type='transformer'
```
- Uses self-attention for cross-modal learning
- Higher computational cost
- Better for complex multi-modal interactions

#### 3. Concat Fusion
```python
fusion_type='concat'
```
- Simple concatenation with convolution
- Lowest computational cost
- Baseline method

## Architecture Comparison

| Feature | SkySensePP | Unet | Unet++ | DeepLabV3+ |
|---------|-----------|------|--------|------------|
| Multi-modal Support | ✓ | ✗ | ✗ | ✗ |
| Semantic Enhancement | ✓ | ✗ | ✗ | ✗ |
| Adaptive Fusion | ✓ | ✗ | ✗ | ✗ |
| Channel Attention | ✓ | ✗ | ✗ | ✗ |
| Spatial Attention | ✓ | ✗ | ✗ | ✗ |
| Transformer Fusion | ✓ | ✗ | ✗ | ✗ |

## Model Parameters

Typical model size comparison (ResNet34 encoder, 4 input channels):

| Model | Parameters | Input Size | Output Size |
|-------|------------|------------|-------------|
| Unet | ~24M | 256×256×4 | 256×256×1 |
| Unet++ | ~47M | 256×256×4 | 256×256×1 |
| DeepLabV3+ | ~59M | 256×256×4 | 256×256×1 |
| **SkySensePP** | ~27M | 256×256×4 | 256×256×1 |

## Testing

Run the test script:

```bash
python test_skysense_pp.py
```

Or check syntax:

```bash
python check_syntax.py
```

## Implementation Details

### Dependencies

The implementation uses:
- `segmentation_models_pytorch` for encoder backbones
- Standard PyTorch for neural network components
- Custom fusion and attention mechanisms

### Input Requirements

- **Channels**: 3 (RGB) or 4 (RGB + NIR)
- **Spatial Size**: Arbitrary (power of 2 recommended)
- **Data Type**: Float32 tensor, normalized to [0, 1]

### Output

- **Channels**: Number of classes (typically 1 for binary segmentation)
- **Size**: Same spatial size as input
- **Activation**: Sigmoid (default) or Softmax based on configuration

## Limitations

1. **Computational Cost**: Transformer fusion is more computationally expensive
2. **Memory Usage**: Semantic enhancement modules increase memory footprint
3. **Pretrained Weights**: Currently supports ImageNet pretrained encoders only
4. **Multi-modal Input**: Optimized for RGB+NIR, other modalities need adaptation

## Future Improvements

- [ ] Add support for Sentinel-1 and Sentinel-2 time series
- [ ] Implement CTPE (Calendar Time Positional Encoding)
- [ ] Add modality VAE for cross-modal generation
- [ ] Support for loading pretrained SkySense++ weights
- [ ] Multi-scale inference and test-time augmentation

## Citation

If you use this implementation in your research, please cite:

```bibtex
@article{wu2025semantic,
  author = {Wu, Kang and Zhang, Yingying and Ru, Lixiang and Dang, Bo and 
            Lao, Jiangwei and Yu, Lei and Luo, Junwei and Zhu, Zifan and 
            Sun, Yue and Zhang, Jiahao and Zhu, Qi and Wang, Jian and 
            Yang, Ming and Chen, Jingdong and Zhang, Yongjun and Li, Yansheng},
  title = {A semantic‑enhanced multi‑modal remote sensing foundation model 
          for Earth observation},
  journal = {Nature Machine Intelligence},
  year = {2025},
  doi = {10.1038/s42256-025-01078-8}
}
```

## License

This integration follows the license terms of the original SkySense++ project.
The pre-trained model weights and pre-training code are only available for 
non-commercial research. For commercial use, please contact the original authors.

## Contact

For issues or questions about the SkySense++ integration:
1. Check the original [SkySense++ repository](https://github.com/kang-wu/SkySensePlusPlus)
2. Review this project's [GitHub Issues](../../issues)
3. Consult the main [README.md](README.md)
