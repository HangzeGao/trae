"""
SkySense++ model integration for cloud segmentation.
Based on SkySense++: A Semantic-Enhanced Multi-Modal Remote Sensing Foundation Model
Reference: https://github.com/kang-wu/SkySensePlusPlus
"""
import torch
import torch.nn as nn
import torch.nn.functional as F
from typing import Dict, List, Optional, Tuple, Union
import math

try:
    from utils.config import Config
    HAS_CONFIG = True
except ImportError:
    HAS_CONFIG = False


class ConvModule(nn.Module):
    """Convolution module with batch normalization and activation."""
    
    def __init__(
        self,
        in_channels: int,
        out_channels: int,
        kernel_size: int = 3,
        stride: int = 1,
        padding: int = 1,
        dilation: int = 1,
        groups: int = 1,
        bias: bool = False,
        act_cfg: dict = {'type': 'ReLU', 'inplace': True}
    ):
        super().__init__()
        
        self.conv = nn.Conv2d(
            in_channels,
            out_channels,
            kernel_size,
            stride,
            padding,
            dilation,
            groups,
            bias
        )
        self.bn = nn.BatchNorm2d(out_channels)
        self.act = self._build_activation(act_cfg)
    
    def _build_activation(self, act_cfg: dict) -> nn.Module:
        act_type = act_cfg.get('type', 'ReLU')
        if act_type == 'ReLU':
            return nn.ReLU(inplace=act_cfg.get('inplace', True))
        elif act_type == 'GELU':
            return nn.GELU()
        elif act_type == 'SiLU':
            return nn.SiLU(inplace=act_cfg.get('inplace', True))
        elif act_type == 'Sigmoid':
            return nn.Sigmoid()
        else:
            return nn.ReLU(inplace=True)
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        x = self.conv(x)
        x = self.bn(x)
        if self.act is not None:
            x = self.act(x)
        return x


class MultiModalFusion(nn.Module):
    """
    Multi-modal feature fusion module inspired by SkySense++.
    Fuses features from multiple modalities (RGB, NIR, SAR, etc.)
    """
    
    def __init__(
        self,
        in_channels_list: List[int],
        out_channels: int,
        fusion_type: str = 'adaptive',
        use_modality_vae: bool = False
    ):
        super().__init__()
        
        self.in_channels_list = in_channels_list
        self.out_channels = out_channels
        self.fusion_type = fusion_type
        self.use_modality_vae = use_modality_vae
        
        self.input_convs = nn.ModuleList()
        for in_ch in in_channels_list:
            self.input_convs.append(
                ConvModule(in_ch, out_channels, kernel_size=1, padding=0)
            )
        
        if fusion_type == 'adaptive':
            self.fusion_conv = nn.Sequential(
                ConvModule(out_channels * len(in_channels_list), out_channels),
                ConvModule(out_channels, out_channels)
            )
            self.attention = nn.Sequential(
                nn.AdaptiveAvgPool2d(1),
                nn.Conv2d(out_channels * len(in_channels_list), out_channels, 1),
                nn.Sigmoid()
            )
        elif fusion_type == 'transformer':
            self.fusion_conv = TransformerFusion(
                out_channels,
                num_heads=8,
                num_layers=2
            )
        else:
            self.fusion_conv = nn.Conv2d(
                out_channels * len(in_channels_list),
                out_channels,
                1
            )
    
    def forward(
        self,
        features: List[torch.Tensor]
    ) -> torch.Tensor:
        """
        Args:
            features: List of feature tensors from different modalities
        Returns:
            Fused feature tensor
        """
        assert len(features) == len(self.in_channels_list)
        
        transformed = [
            conv(feat) for conv, feat in zip(self.input_convs, features)
        ]
        
        if self.fusion_type == 'adaptive':
            concat_feat = torch.cat(transformed, dim=1)
            attention = self.attention(concat_feat)
            fused = self.fusion_conv(concat_feat)
            fused = fused * attention
        elif self.fusion_type == 'transformer':
            B, C, H, W = transformed[0].shape
            seq_len = H * W
            concat_feat = torch.cat(transformed, dim=1)
            concat_feat = concat_feat.view(B, -1, seq_len).permute(0, 2, 1)
            fused = self.fusion_conv(concat_feat)
            fused = fused.permute(0, 2, 1).view(B, -1, H, W)
        else:
            concat_feat = torch.cat(transformed, dim=1)
            fused = self.fusion_conv(concat_feat)
        
        return fused


class TransformerFusion(nn.Module):
    """Transformer-based fusion module."""
    
    def __init__(
        self,
        embed_dim: int,
        num_heads: int = 8,
        num_layers: int = 2,
        dropout: float = 0.1
    ):
        super().__init__()
        
        encoder_layer = nn.TransformerEncoderLayer(
            d_model=embed_dim,
            nhead=num_heads,
            dim_feedforward=embed_dim * 4,
            dropout=dropout,
            activation='gelu',
            batch_first=True
        )
        self.transformer = nn.TransformerEncoder(
            encoder_layer,
            num_layers=num_layers
        )
        self.proj = nn.Linear(embed_dim, embed_dim)
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.transformer(x)


class SemanticEnhancedDecoder(nn.Module):
    """
    Semantic-enhanced decoder for high-resolution segmentation.
    Inspired by SkySense++ semantic enhancement approach.
    """
    
    def __init__(
        self,
        encoder_channels: List[int],
        decoder_channels: List[int],
        num_classes: int,
        use_skip_connections: bool = True,
        dropout: float = 0.1
    ):
        super().__init__()
        
        self.encoder_channels = encoder_channels
        self.decoder_channels = decoder_channels
        self.num_classes = num_classes
        self.use_skip_connections = use_skip_connections
        
        self.decoder_blocks = nn.ModuleList()
        for i in range(len(decoder_channels)):
            in_ch = encoder_channels[-1] if i == 0 else decoder_channels[i - 1]
            out_ch = decoder_channels[i]
            
            self.decoder_blocks.append(
                nn.Sequential(
                    ConvModule(in_ch, out_ch),
                    ConvModule(out_ch, out_ch)
                )
            )
        
        self.upsample_blocks = nn.ModuleList([
            nn.ConvTranspose2d(
                decoder_channels[i],
                decoder_channels[i],
                kernel_size=2,
                stride=2
            ) for i in range(len(decoder_channels) - 1)
        ])
        
        self.final_conv = nn.Sequential(
            ConvModule(decoder_channels[-1], decoder_channels[-1] // 2),
            nn.Dropout2d(dropout),
            nn.Conv2d(decoder_channels[-1] // 2, num_classes, 1)
        )
    
    def forward(
        self,
        encoder_features: List[torch.Tensor]
    ) -> torch.Tensor:
        """
        Args:
            encoder_features: List of encoder feature maps (from low to high level)
        Returns:
            Segmentation logits
        """
        x = encoder_features[-1]
        
        for i, decoder_block in enumerate(self.decoder_blocks):
            x = decoder_block(x)
            
            if i < len(self.upsample_blocks):
                x = self.upsample_blocks[i](x)
                
                if self.use_skip_connections and i < len(encoder_features) - 1:
                    skip_idx = -(i + 2)
                    skip = encoder_features[skip_idx]
                    
                    if x.shape != skip.shape:
                        x = F.interpolate(
                            x,
                            size=skip.shape[2:],
                            mode='bilinear',
                            align_corners=True
                        )
                    
                    x = x + skip
        
        logits = self.final_conv(x)
        return logits


class SkySensePPModel(nn.Module):
    """
    SkySense++ inspired model for cloud segmentation.
    
    This is a simplified implementation adapted from SkySense++ for cloud segmentation
    with support for multi-modal inputs (RGB + NIR).
    
    Reference:
        Wu et al. "SkySense++: A Semantic-Enhanced Multi-Modal Remote Sensing Foundation Model
        Beyond SkySense for Earth Observation" (Nature Machine Intelligence, 2025)
    """
    
    def __init__(
        self,
        encoder_name: str = 'resnet34',
        encoder_depth: int = 5,
        encoder_channels: List[int] = None,
        decoder_channels: List[int] = [256, 128, 64, 32, 16],
        in_channels: int = 4,
        num_classes: int = 1,
        fusion_type: str = 'adaptive',
        use_semantic_enhancement: bool = True,
        dropout: float = 0.1,
        activation: str = 'sigmoid'
    ):
        super().__init__()
        
        if encoder_channels is None:
            encoder_channels = [64, 128, 256, 512, 1024]
        
        self.encoder_channels = encoder_channels[:encoder_depth]
        self.decoder_channels = decoder_channels[:encoder_depth]
        self.num_classes = num_classes
        self.activation = activation
        
        if in_channels > 3:
            self.input_proj = ConvModule(in_channels, 3)
            self.in_channels = 3
        else:
            self.input_proj = None
            self.in_channels = in_channels
        
        try:
            import segmentation_models_pytorch as smp
            self.encoder = smp.encoders.get_encoder(
                encoder_name,
                in_channels=self.in_channels,
                depth=encoder_depth,
                weights='imagenet' if encoder_name.startswith('resnet') else None
            )
            self.use_smp_encoder = True
        except:
            from models.unet import create_model
            base_model = create_model(
                architecture='Unet',
                encoder_name=encoder_name,
                encoder_weights='imagenet' if encoder_name.startswith('resnet') else None,
                in_channels=self.in_channels,
                classes=32,
                activation='relu'
            )
            self.encoder = baseModelWrapper(base_model.encoder)
            self.use_smp_encoder = False
        
        self.fusion = MultiModalFusion(
            in_channels_list=[self.encoder_channels[-1]],
            out_channels=self.decoder_channels[0],
            fusion_type=fusion_type,
            use_modality_vae=False
        )
        
        if use_semantic_enhancement:
            self.semantic_enhancer = SemanticEnhancementModule(
                channels=self.decoder_channels[0]
            )
        else:
            self.semantic_enhancer = None
        
        self.decoder = SemanticEnhancedDecoder(
            encoder_channels=self.encoder_channels,
            decoder_channels=self.decoder_channels,
            num_classes=num_classes,
            dropout=dropout
        )
        
        self._init_weights()
    
    def _init_weights(self):
        for m in self.modules():
            if isinstance(m, nn.Conv2d):
                nn.init.kaiming_normal_(m.weight, mode='fan_out', nonlinearity='relu')
                if m.bias is not None:
                    nn.init.constant_(m.bias, 0)
            elif isinstance(m, nn.BatchNorm2d):
                nn.init.constant_(m.weight, 1)
                nn.init.constant_(m.bias, 0)
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        if self.input_proj is not None:
            x = self.input_proj(x)
        
        if self.use_smp_encoder:
            features = self.encoder(x)
        else:
            features = self.encoder(x)
        
        fused = self.fusion([features[-1]])
        
        if self.semantic_enhancer is not None:
            fused = self.semantic_enhancer(fused)
        
        logits = self.decoder(features)
        
        if self.activation == 'sigmoid':
            logits = torch.sigmoid(logits)
        elif self.activation == 'softmax':
            logits = F.softmax(logits, dim=1)
        
        return logits


class SemanticEnhancementModule(nn.Module):
    """Semantic enhancement module for improving feature representations."""
    
    def __init__(self, channels: int):
        super().__init__()
        
        self.context_aggregation = nn.Sequential(
            nn.AdaptiveAvgPool2d(1),
            nn.Conv2d(channels, channels // 4, 1),
            nn.ReLU(inplace=True),
            nn.Conv2d(channels // 4, channels, 1),
            nn.Sigmoid()
        )
        
        self.spatial_attention = SpatialAttentionModule(channels)
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        channel_attn = self.context_aggregation(x)
        x = x * channel_attn
        x = self.spatial_attention(x)
        return x


class SpatialAttentionModule(nn.Module):
    """Spatial attention module for focusing on important regions."""
    
    def __init__(self, channels: int):
        super().__init__()
        
        self.conv = nn.Sequential(
            nn.Conv2d(channels, channels // 4, 1),
            nn.ReLU(inplace=True),
            nn.Conv2d(channels // 4, 1, 1),
            nn.Sigmoid()
        )
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        attention = self.conv(x)
        return x * attention


class baseModelWrapper(nn.Module):
    """Wrapper for base segmentation models to extract encoder features."""
    
    def __init__(self, encoder):
        super().__init__()
        self.encoder = encoder
    
    def forward(self, x):
        return self.encoder(x)


def create_skysense_pp_model_from_config(config: Union[Config, dict]) -> nn.Module:
    """
    Create SkySense++ model from configuration.
    
    Args:
        config: Configuration object or dict
    
    Returns:
        SkySensePPModel instance
    """
    if HAS_CONFIG and isinstance(config, Config):
        model_cfg = config.model
    else:
        model_cfg = config.get('model', {})
    
    architecture = model_cfg.get('architecture', 'SkySensePP')
    
    if architecture != 'SkySensePP':
        from models.unet import create_model_from_config
        return create_model_from_config(config)
    
    encoder_name = model_cfg.get('encoder_name', 'resnet34')
    encoder_depth = model_cfg.get('encoder_depth', 5)
    encoder_channels = model_cfg.get('encoder_channels', [64, 128, 256, 512, 1024])
    decoder_channels = model_cfg.get('decoder_channels', [256, 128, 64, 32, 16])
    in_channels = model_cfg.get('in_channels', 4)
    num_classes = model_cfg.get('out_channels', 1)
    fusion_type = model_cfg.get('fusion_type', 'adaptive')
    use_semantic_enhancement = model_cfg.get('use_semantic_enhancement', True)
    dropout = model_cfg.get('dropout', 0.1)
    activation = model_cfg.get('activation', 'sigmoid')
    
    return SkySensePPModel(
        encoder_name=encoder_name,
        encoder_depth=encoder_depth,
        encoder_channels=encoder_channels,
        decoder_channels=decoder_channels,
        in_channels=in_channels,
        num_classes=num_classes,
        fusion_type=fusion_type,
        use_semantic_enhancement=use_semantic_enhancement,
        dropout=dropout,
        activation=activation
    )


if __name__ == '__main__':
    print("Testing SkySense++ model creation...")
    
    model = SkySensePPModel(
        encoder_name='resnet34',
        in_channels=4,
        num_classes=1,
        fusion_type='adaptive',
        use_semantic_enhancement=True
    )
    
    x = torch.randn(2, 4, 256, 256)
    y = model(x)
    
    print(f"Input shape: {x.shape}")
    print(f"Output shape: {y.shape}")
    print(f"Total parameters: {sum(p.numel() for p in model.parameters()):,}")
    
    test_config = {
        'model': {
            'architecture': 'SkySensePP',
            'encoder_name': 'resnet34',
            'encoder_depth': 5,
            'in_channels': 4,
            'out_channels': 1,
            'decoder_channels': [256, 128, 64, 32, 16],
            'fusion_type': 'adaptive',
            'use_semantic_enhancement': True,
            'dropout': 0.1,
            'activation': 'sigmoid'
        }
    }
    
    model2 = create_skysense_pp_model_from_config(test_config)
    y2 = model2(x)
    print(f"\nModel from config - Output shape: {y2.shape}")
