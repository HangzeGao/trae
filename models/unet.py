import torch
import torch.nn as nn


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
        
        # Handle different image sizes due to padding
        diff_y = x2.size()[2] - x1.size()[2]
        diff_x = x2.size()[3] - x1.size()[3]
        
        x1 = nn.functional.pad(x1, [diff_x // 2, diff_x - diff_x // 2,
                                     diff_y // 2, diff_y - diff_y // 2])
        
        x = torch.cat([x2, x1], dim=1)
        return self.conv(x)


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
