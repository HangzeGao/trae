#!/usr/bin/env python3
"""
Simplified SkySense++ model for quick testing.
"""

import torch
import torch.nn as nn
import torch.nn.functional as F
import segmentation_models_pytorch as smp


class SimplifiedSkySensePP(nn.Module):
    """Simplified SkySense++ inspired model."""
    
    def __init__(self, encoder_name='resnet34', in_channels=4, num_classes=1):
        super().__init__()
        
        # Input projection
        if in_channels != 3:
            self.input_proj = nn.Conv2d(in_channels, 3, kernel_size=1)
        else:
            self.input_proj = None
        
        # Encoder (using segmentation_models_pytorch)
        self.encoder = smp.encoders.get_encoder(
            encoder_name,
            in_channels=3,
            depth=5,
            weights='imagenet'
        )
        
        # Decoder (simple U-Net style)
        encoder_channels = [64, 128, 256, 512, 1024]  # ResNet34
        decoder_channels = [256, 128, 64, 32, 16]
        
        self.decoder_blocks = nn.ModuleList()
        self.upsamples = nn.ModuleList()
        
        for i in range(len(decoder_channels)):
            in_ch = encoder_channels[-1] if i == 0 else decoder_channels[i-1]
            self.decoder_blocks.append(
                nn.Sequential(
                    nn.Conv2d(in_ch, decoder_channels[i], 3, padding=1),
                    nn.BatchNorm2d(decoder_channels[i]),
                    nn.ReLU(inplace=True),
                    nn.Conv2d(decoder_channels[i], decoder_channels[i], 3, padding=1),
                    nn.BatchNorm2d(decoder_channels[i]),
                    nn.ReLU(inplace=True)
                )
            )
            
            if i < len(decoder_channels) - 1:
                self.upsamples.append(
                    nn.ConvTranspose2d(decoder_channels[i], decoder_channels[i], 2, stride=2)
                )
        
        # Final output
        self.final_conv = nn.Conv2d(decoder_channels[-1], num_classes, 1)
        
        # Semantic enhancement
        self.channel_attention = nn.Sequential(
            nn.AdaptiveAvgPool2d(1),
            nn.Conv2d(decoder_channels[0], decoder_channels[0] // 4, 1),
            nn.ReLU(),
            nn.Conv2d(decoder_channels[0] // 4, decoder_channels[0], 1),
            nn.Sigmoid()
        )
    
    def forward(self, x):
        if self.input_proj is not None:
            x = self.input_proj(x)
        
        # Encoder
        features = self.encoder(x)  # List of feature maps
        
        # Decoder
        x = features[-1]  # Start with deepest features
        
        for i, (decoder_block, upsample) in enumerate(zip(self.decoder_blocks[:-1], self.upsamples)):
            x = decoder_block(x)
            x = upsample(x)
            
            # Skip connection
            skip = features[-(i+2)]
            if x.shape != skip.shape:
                x = F.interpolate(x, size=skip.shape[2:], mode='bilinear', align_corners=True)
            
            # Handle channel mismatch
            if x.shape[1] != skip.shape[1]:
                skip = nn.Conv2d(skip.shape[1], x.shape[1], 1).to(x.device)(skip)
            
            x = x + skip
        
        # Final decoder block
        x = self.decoder_blocks[-1](x)
        
        # Semantic enhancement
        attention = self.channel_attention(x)
        x = x * attention
        
        # Output
        x = self.final_conv(x)
        x = torch.sigmoid(x)
        
        return x


if __name__ == '__main__':
    print("Testing Simplified SkySense++...")
    
    model = SimplifiedSkySensePP(encoder_name='resnet34', in_channels=4, num_classes=1)
    
    # Test forward pass
    x = torch.randn(2, 4, 256, 256)
    y = model(x)
    
    print(f"Input: {x.shape}")
    print(f"Output: {y.shape}")
    print(f"Parameters: {sum(p.numel() for p in model.parameters()):,}")
    
    print("\n✓ Model test passed!")
