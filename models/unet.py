import torch
import torch.nn as nn
import torchvision.models as models


class DoubleConv(nn.Module):
    """Double Convolution block with BatchNorm and ReLU"""
    
    def __init__(self, in_channels, out_channels):
        super(DoubleConv, self).__init__()
        self.double_conv = nn.Sequential(
            nn.Conv2d(in_channels, out_channels, kernel_size=3, padding=1),
            nn.BatchNorm2d(out_channels),
            nn.ReLU(inplace=True),
            nn.Conv2d(out_channels, out_channels, kernel_size=3, padding=1),
            nn.BatchNorm2d(out_channels),
            nn.ReLU(inplace=True),
        )
    
    def forward(self, x):
        return self.double_conv(x)


class Down(nn.Module):
    """Downsampling path (encoder)"""
    
    def __init__(self, in_channels, out_channels):
        super(Down, self).__init__()
        self.maxpool_conv = nn.Sequential(
            nn.MaxPool2d(2),
            DoubleConv(in_channels, out_channels)
        )
    
    def forward(self, x):
        return self.maxpool_conv(x)


class Up(nn.Module):
    """Upsampling path (decoder)"""
    
    def __init__(self, in_channels, out_channels, bilinear=True):
        super(Up, self).__init__()
        if bilinear:
            self.up = nn.Upsample(scale_factor=2, mode='bilinear', align_corners=True)
            self.conv = DoubleConv(in_channels, out_channels)
        else:
            self.up = nn.ConvTranspose2d(in_channels // 2, in_channels // 2, kernel_size=2, stride=2)
            self.conv = DoubleConv(in_channels, out_channels)
    
    def forward(self, x1, x2):
        x1 = self.up(x1)
        
        diff_y = x2.size()[2] - x1.size()[2]
        diff_x = x2.size()[3] - x1.size()[3]
        
        x1 = nn.functional.pad(x1, [diff_x // 2, diff_x - diff_x // 2,
                                     diff_y // 2, diff_y - diff_y // 2])
        
        x = torch.cat([x2, x1], dim=1)
        return self.conv(x)


class DecoderBlock(nn.Module):
    """Decoder block with upsampling and convolution"""
    
    def __init__(self, in_channels, skip_channels, out_channels):
        super(DecoderBlock, self).__init__()
        self.up = nn.Upsample(scale_factor=2, mode='bilinear', align_corners=True)
        self.conv = DoubleConv(in_channels + skip_channels, out_channels)
    
    def forward(self, x, skip):
        x = self.up(x)
        
        diff_y = skip.size()[2] - x.size()[2]
        diff_x = skip.size()[3] - x.size()[3]
        
        x = nn.functional.pad(x, [diff_x // 2, diff_x - diff_x // 2,
                                  diff_y // 2, diff_y - diff_y // 2])
        
        x = torch.cat([skip, x], dim=1)
        return self.conv(x)


class ResNetEncoder(nn.Module):
    """
    ResNet encoder for U-Net with pretrained weights
    
    Supports: resnet18, resnet34, resnet50, resnet101, efficientnet_b4, mobilenet_v2
    """
    
    def __init__(self, backbone_name='resnet34', pretrained=True):
        super(ResNetEncoder, self).__init__()
        
        if backbone_name == 'resnet34':
            backbone = models.resnet34(weights='IMAGENET1K_V1' if pretrained else None)
        elif backbone_name == 'resnet50':
            backbone = models.resnet50(weights='IMAGENET1K_V1' if pretrained else None)
        elif backbone_name == 'resnet18':
            backbone = models.resnet18(weights='IMAGENET1K_V1' if pretrained else None)
        elif backbone_name == 'resnet101':
            backbone = models.resnet101(weights='IMAGENET1K_V1' if pretrained else None)
        elif backbone_name == 'efficientnet_b4':
            backbone = models.efficientnet_b4(weights='IMAGENET1K_V1' if pretrained else None)
        elif backbone_name == 'mobilenet_v2':
            backbone = models.mobilenet_v2(weights='IMAGENET1K_V1' if pretrained else None)
        else:
            raise ValueError(f"Unsupported backbone: {backbone_name}")
        
        self.backbone_name = backbone_name
        
        if 'efficientnet' in backbone_name:
            self.initial = nn.Sequential(
                backbone.features[0],
                backbone.features[1],
                backbone.features[2:4],
                backbone.features[4:6],
                backbone.features[6:9],
                backbone.features[9:],
            )
            self.out_channels = [3, 24, 32, 56, 112, 1280]
        elif backbone_name == 'mobilenet_v2':
            self.initial = nn.Sequential(
                backbone.features[0],
                backbone.features[1:3],
                backbone.features[3:6],
                backbone.features[6:9],
                backbone.features[9:12],
                backbone.features[12:14],
                backbone.features[14:17],
                backbone.features[17:],
            )
            self.out_channels = [3, 16, 24, 32, 64, 96, 160, 320]
        else:
            self.initial = nn.Sequential(
                backbone.conv1,
                backbone.bn1,
                backbone.relu,
                backbone.maxpool,
                backbone.layer1,
                backbone.layer2,
                backbone.layer3,
                backbone.layer4,
            )
            
            if backbone_name in ['resnet18', 'resnet34']:
                self.out_channels = [64, 64, 128, 256, 512]
            else:
                self.out_channels = [64, 256, 512, 1024, 2048]
    
    def forward(self, x):
        if 'efficientnet' in self.backbone_name:
            features = []
            for i, layer in enumerate(self.initial):
                x = layer(x)
                if i > 0:
                    features.append(x)
            return features
        elif self.backbone_name == 'mobilenet_v2':
            features = []
            for layer in self.initial:
                x = layer(x)
                features.append(x)
            return features[1:]
        else:
            x0 = self.initial[0:3](x)
            x1 = self.initial[3:4](x0)
            x2 = self.initial[4:5](x1)
            x3 = self.initial[5:6](x2)
            x4 = self.initial[6:7](x3)
            return [x1, x2, x3, x4]


class UNetWithBackbone(nn.Module):
    """
    U-Net architecture with pretrained backbone encoder
    
    Args:
        backbone_name: Encoder backbone ('resnet34', 'resnet50', 'efficientnet_b4', 'mobilenet_v2')
        num_classes: Number of output classes
        pretrained: Use pretrained ImageNet weights
    """
    
    def __init__(self, backbone_name='resnet34', num_classes=1, pretrained=True):
        super(UNetWithBackbone, self).__init__()
        
        self.encoder = ResNetEncoder(backbone_name, pretrained)
        encoder_channels = self.encoder.out_channels
        
        if 'efficientnet' in backbone_name:
            decoder_channels = [560, 288, 144, 64, 32]
            center_channels = encoder_channels[-1]
        elif backbone_name == 'mobilenet_v2':
            decoder_channels = [160, 96, 64, 32, 24]
            center_channels = encoder_channels[-1]
        else:
            decoder_channels = [256, 128, 64, 32, 16]
            center_channels = encoder_channels[-1]
        
        self.center = nn.Sequential(
            nn.Conv2d(center_channels, decoder_channels[0], 3, padding=1),
            nn.BatchNorm2d(decoder_channels[0]),
            nn.ReLU(inplace=True),
            nn.Conv2d(decoder_channels[0], decoder_channels[0], 3, padding=1),
            nn.BatchNorm2d(decoder_channels[0]),
            nn.ReLU(inplace=True),
        )
        
        self.decoder_blocks = nn.ModuleList()
        in_ch = decoder_channels[0]
        for i, out_ch in enumerate(decoder_channels[1:]):
            skip_ch = encoder_channels[-(i+2)]
            self.decoder_blocks.append(DecoderBlock(in_ch, skip_ch, out_ch))
            in_ch = out_ch
        
        self.final_conv = nn.Conv2d(decoder_channels[-1], num_classes, kernel_size=1)
        self.sigmoid = nn.Sigmoid()
        
        self.backbone_name = backbone_name
    
    def forward(self, x):
        features = self.encoder(x)
        
        x = self.center(features[-1])
        
        for i, decoder_block in enumerate(self.decoder_blocks):
            skip = features[-(i+2)]
            x = decoder_block(x, skip)
        
        x = self.final_conv(x)
        return self.sigmoid(x)


class UNet(nn.Module):
    """
    UNet architecture for image segmentation
    
    U-Net: Convolutional Networks for Biomedical Image Segmentation
    (Ronneberger, Fischer, and Brox, 2015)
    """
    
    def __init__(self, in_channels=3, out_channels=1, init_features=64, bilinear=True):
        """
        Args:
            in_channels: Number of input channels (default: 3 for RGB)
            out_channels: Number of output channels (default: 1 for binary segmentation)
            init_features: Number of features in first conv layer (default: 64)
            bilinear: Use bilinear upsampling or ConvTranspose2d (default: True)
        """
        super(UNet, self).__init__()
        
        self.in_channels = in_channels
        self.out_channels = out_channels
        self.bilinear = bilinear
        
        features = init_features
        
        # Encoder (downsampling)
        self.inc = DoubleConv(in_channels, features)
        self.down1 = Down(features, features * 2)
        self.down2 = Down(features * 2, features * 4)
        self.down3 = Down(features * 4, features * 8)
        self.down4 = Down(features * 8, features * 16)
        
        # Decoder (upsampling)
        self.up1 = Up(features * 16, features * 8, bilinear)
        self.up2 = Up(features * 8, features * 4, bilinear)
        self.up3 = Up(features * 4, features * 2, bilinear)
        self.up4 = Up(features * 2, features, bilinear)
        
        # Final output layer
        self.outc = nn.Sequential(
            nn.Conv2d(features, out_channels, kernel_size=1),
            nn.Sigmoid()  # For binary segmentation
        )
    
    def forward(self, x):
        # Encoder
        x1 = self.inc(x)
        x2 = self.down1(x1)
        x3 = self.down2(x2)
        x4 = self.down3(x3)
        x5 = self.down4(x4)
        
        # Decoder with skip connections
        x = self.up1(x5, x4)
        x = self.up2(x, x3)
        x = self.up3(x, x2)
        x = self.up4(x, x1)
        
        # Output
        x = self.outc(x)
        return x


def create_unet(in_channels=3, out_channels=1, init_features=64):
    """Factory function to create UNet model"""
    return UNet(in_channels, out_channels, init_features)


if __name__ == "__main__":
    # Test the model
    model = UNet(in_channels=3, out_channels=1, init_features=64)
    x = torch.randn(1, 3, 256, 256)
    y = model(x)
    print(f"Input shape: {x.shape}")
    print(f"Output shape: {y.shape}")
    print(f"Total parameters: {sum(p.numel() for p in model.parameters()):,}")
