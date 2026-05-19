import torch
import torch.nn as nn
import torch.nn.functional as F
from typing import Dict, List, Optional, Tuple


class VisionTransformer(nn.Module):
    """Vision Transformer encoder for spatial feature extraction."""
    
    def __init__(self, in_channels: int = 4, embed_dim: int = 256, num_heads: int = 8, 
                 num_layers: int = 6, patch_size: int = 16):
        super().__init__()
        self.patch_size = patch_size
        self.embed_dim = embed_dim
        
        self.patch_embedding = nn.Conv2d(in_channels, embed_dim, kernel_size=patch_size, stride=patch_size)
        
        self.pos_embedding = nn.Parameter(torch.randn(1, 1024 + 1, embed_dim))
        self.cls_token = nn.Parameter(torch.randn(1, 1, embed_dim))
        
        encoder_layer = nn.TransformerEncoderLayer(
            d_model=embed_dim, nhead=num_heads, dim_feedforward=embed_dim * 4,
            activation='gelu', batch_first=True, norm_first=True
        )
        self.transformer = nn.TransformerEncoder(encoder_layer, num_layers=num_layers)
        
        self.norm = nn.LayerNorm(embed_dim)
    
    def forward(self, x: torch.Tensor) -> Tuple[torch.Tensor, torch.Tensor]:
        """
        Args:
            x: Input tensor of shape (B, C, H, W)
        
        Returns:
            cls_token: Global feature of shape (B, embed_dim)
            patch_tokens: Patch features of shape (B, num_patches, embed_dim)
        """
        B, C, H, W = x.shape
        
        patches = self.patch_embedding(x)
        patches = patches.flatten(2).transpose(1, 2)
        
        cls_token = self.cls_token.expand(B, -1, -1)
        x = torch.cat([cls_token, patches], dim=1)
        
        pos_embed = self.pos_embedding[:, :x.shape[1]]
        x = x + pos_embed
        
        x = self.transformer(x)
        x = self.norm(x)
        
        return x[:, 0], x[:, 1:]


class SpatioTemporalEncoder(nn.Module):
    """Spatio-Temporal Modality Decoupling Encoder."""
    
    def __init__(self, in_channels: int = 4, embed_dim: int = 256, num_heads: int = 8, 
                 num_layers: int = 6, patch_size: int = 16):
        super().__init__()
        
        self.spatial_encoder = VisionTransformer(
            in_channels=in_channels,
            embed_dim=embed_dim,
            num_heads=num_heads,
            num_layers=num_layers,
            patch_size=patch_size
        )
        
        self.channel_attention = nn.Sequential(
            nn.Conv2d(in_channels, in_channels // 2, kernel_size=1),
            nn.ReLU(),
            nn.Conv2d(in_channels // 2, in_channels, kernel_size=1),
            nn.Sigmoid()
        )
    
    def forward(self, x: torch.Tensor) -> Tuple[torch.Tensor, torch.Tensor]:
        """
        Args:
            x: Input tensor of shape (B, C, H, W)
        
        Returns:
            global_feature: Global feature of shape (B, embed_dim)
            local_features: Local features of shape (B, num_patches, embed_dim)
        """
        channel_att = self.channel_attention(x)
        x = x * channel_att
        
        global_feature, local_features = self.spatial_encoder(x)
        
        return global_feature, local_features


class MultiGranularityFusion(nn.Module):
    """Multi-granularity feature fusion module."""
    
    def __init__(self, embed_dim: int = 256, num_heads: int = 8):
        super().__init__()
        
        self.pixel_fusion = nn.Conv2d(embed_dim, embed_dim, kernel_size=3, padding=1)
        self.object_fusion = nn.MultiheadAttention(embed_dim, num_heads, batch_first=True)
        self.image_fusion = nn.Linear(embed_dim, embed_dim)
        
        self.norm = nn.LayerNorm(embed_dim)
    
    def forward(self, local_features: torch.Tensor, global_feature: torch.Tensor, 
                H: int, W: int, patch_size: int = 16) -> torch.Tensor:
        """
        Args:
            local_features: Patch features of shape (B, num_patches, embed_dim)
            global_feature: Global feature of shape (B, embed_dim)
            H: Original height
            W: Original width
        
        Returns:
            fused_feature: Fused feature map of shape (B, embed_dim, H, W)
        """
        B, num_patches, embed_dim = local_features.shape
        
        num_patches_h = H // patch_size
        num_patches_w = W // patch_size
        
        pixel_features = local_features.transpose(1, 2).view(B, embed_dim, num_patches_h, num_patches_w)
        pixel_features = self.pixel_fusion(pixel_features)
        pixel_features = F.interpolate(pixel_features, size=(H, W), mode='bilinear', align_corners=False)
        
        global_expanded = global_feature.unsqueeze(1).expand(B, num_patches, embed_dim)
        object_output, _ = self.object_fusion(local_features, global_expanded, global_expanded)
        
        image_feature = self.image_fusion(global_feature).unsqueeze(-1).unsqueeze(-1)
        image_feature = image_feature.expand(B, embed_dim, H, W)
        
        fused = pixel_features + image_feature
        fused = fused + F.interpolate(
            object_output.transpose(1, 2).view(B, embed_dim, num_patches_h, num_patches_w),
            size=(H, W), mode='bilinear', align_corners=False
        )
        
        return fused


class SegmentationHead(nn.Module):
    """Segmentation head for cloud detection."""
    
    def __init__(self, in_channels: int = 256, out_channels: int = 1, num_layers: int = 3):
        super().__init__()
        
        layers = []
        for i in range(num_layers):
            layers.append(nn.Conv2d(in_channels if i == 0 else in_channels // 2, 
                                    in_channels // 2, kernel_size=3, padding=1))
            layers.append(nn.ReLU())
            layers.append(nn.BatchNorm2d(in_channels // 2))
        
        layers.append(nn.Conv2d(in_channels // 2, out_channels, kernel_size=1))
        layers.append(nn.Sigmoid())
        
        self.head = nn.Sequential(*layers)
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.head(x)


class SkySensePlusPlus(nn.Module):
    """SkySense++: Semantic-Enhanced Multi-Modal Remote Sensing Foundation Model."""
    
    def __init__(self, in_channels: int = 4, embed_dim: int = 256, num_heads: int = 8, 
                 num_layers: int = 6, patch_size: int = 16, out_channels: int = 1):
        super().__init__()
        
        self.encoder = SpatioTemporalEncoder(
            in_channels=in_channels,
            embed_dim=embed_dim,
            num_heads=num_heads,
            num_layers=num_layers,
            patch_size=patch_size
        )
        
        self.fusion = MultiGranularityFusion(
            embed_dim=embed_dim,
            num_heads=num_heads
        )
        
        self.segmentation_head = SegmentationHead(
            in_channels=embed_dim,
            out_channels=out_channels
        )
        
        self.patch_size = patch_size
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        """
        Args:
            x: Input tensor of shape (B, C, H, W)
        
        Returns:
            logits: Segmentation logits of shape (B, out_channels, H, W)
        """
        B, C, H, W = x.shape
        
        global_feature, local_features = self.encoder(x)
        
        fused_feature = self.fusion(local_features, global_feature, H, W, self.patch_size)
        
        logits = self.segmentation_head(fused_feature)
        
        return logits


def create_skysense_model(config=None, **kwargs) -> SkySensePlusPlus:
    """
    Create SkySense++ model from config or kwargs.
    
    Args:
        config: Config object (optional)
        **kwargs: Model parameters
    
    Returns:
        SkySensePlusPlus model
    """
    if config is not None:
        model_cfg = config.model if hasattr(config, 'model') else config
        return SkySensePlusPlus(
            in_channels=model_cfg.get('in_channels', 4),
            embed_dim=model_cfg.get('embed_dim', 256),
            num_heads=model_cfg.get('num_heads', 8),
            num_layers=model_cfg.get('num_layers', 6),
            patch_size=model_cfg.get('patch_size', 16),
            out_channels=model_cfg.get('out_channels', 1),
            **kwargs
        )
    
    return SkySensePlusPlus(**kwargs)


if __name__ == "__main__":
    print("Testing SkySense++ model...")
    
    model = SkySensePlusPlus(in_channels=4, embed_dim=128, num_heads=4, num_layers=3, patch_size=16)
    
    x = torch.randn(2, 4, 256, 256)
    y = model(x)
    
    print(f"Input shape: {x.shape}")
    print(f"Output shape: {y.shape}")
    print(f"Total parameters: {sum(p.numel() for p in model.parameters()):,}")
    
    # Test with different sizes
    x2 = torch.randn(1, 4, 512, 512)
    y2 = model(x2)
    print(f"\nInput shape (512x512): {x2.shape}")
    print(f"Output shape: {y2.shape}")