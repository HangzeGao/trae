"""
Segmentation models using segmentation_models_pytorch library.
"""
import torch
import segmentation_models_pytorch as smp
from typing import Dict, Any, Optional

try:
    from utils.config import Config
    HAS_CONFIG = True
except ImportError:
    HAS_CONFIG = False


def create_model_from_config(config: Config, **kwargs) -> torch.nn.Module:
    """
    Create model from Config object.
    
    Args:
        config: Config object
        **kwargs: Additional model kwargs
    
    Returns:
        PyTorch model
    """
    model_cfg = config.model
    return create_model(
        architecture=model_cfg.get('architecture', 'Unet'),
        encoder_name=model_cfg.get('encoder_name', 'resnet34'),
        encoder_weights=model_cfg.get('encoder_weights', 'imagenet'),
        in_channels=model_cfg.get('in_channels', 4),
        classes=model_cfg.get('out_channels', 1),
        activation=model_cfg.get('activation', 'sigmoid'),
        **{**model_cfg.get('model_kwargs', {}), **kwargs}
    )


def create_model(
    architecture: str = "Unet",
    encoder_name: str = "resnet34",
    encoder_weights: str = "imagenet",
    in_channels: int = 3,
    classes: int = 1,
    activation: str = "sigmoid",
    **kwargs
):
    """
    Factory function to create segmentation models using SMP library.
    
    Args:
        architecture: Model architecture (Unet, UnetPlusPlus, DeepLabV3, DeepLabV3Plus, FPN, PSPNet, Linknet, MAnet, PAN)
        encoder_name: Encoder backbone name (resnet18, resnet34, resnet50, efficientnet-b0 to efficientnet-b7, etc.)
        encoder_weights: Pretrained weights ("imagenet" or None)
        in_channels: Number of input channels
        classes: Number of output classes
        activation: Activation function for the final layer ("sigmoid", "softmax", "logsoftmax", "tanh", "identity")
        **kwargs: Additional architecture-specific parameters
    
    Returns:
        PyTorch model
    """
    arch_map = {
        "Unet": smp.Unet,
        "UnetPlusPlus": smp.UnetPlusPlus,
        "DeepLabV3": smp.DeepLabV3,
        "DeepLabV3Plus": smp.DeepLabV3Plus,
        "FPN": smp.FPN,
        "PSPNet": smp.PSPNet,
        "Linknet": smp.Linknet,
        "MAnet": smp.MAnet,
        "PAN": smp.PAN,
    }
    
    if architecture not in arch_map:
        raise ValueError(f"Unsupported architecture: {architecture}. Available: {list(arch_map.keys())}")
    
    model_class = arch_map[architecture]
    model = model_class(
        encoder_name=encoder_name,
        encoder_weights=encoder_weights,
        in_channels=in_channels,
        classes=classes,
        activation=activation,
        **kwargs
    )
    
    return model


def get_preprocessing_fn(encoder_name: str, encoder_weights: str = "imagenet"):
    """
    Get preprocessing function for a specific encoder.
    
    Args:
        encoder_name: Encoder name
        encoder_weights: Pretrained weights
    
    Returns:
        Preprocessing function
    """
    return smp.encoders.get_preprocessing_fn(encoder_name, encoder_weights)


# Keep compatibility with old code
class UNet:
    """Compatibility wrapper for old UNet class"""
    def __new__(cls, in_channels=3, out_channels=1, **kwargs):
        return create_model(
            architecture="Unet",
            encoder_name="resnet34",
            encoder_weights=None,
            in_channels=in_channels,
            classes=out_channels,
            activation="sigmoid"
        )


# Keep compatibility with old UNetWithBackbone
class UNetWithBackbone:
    """Compatibility wrapper for old UNetWithBackbone class"""
    def __new__(cls, backbone_name="resnet34", num_classes=1, pretrained=True, **kwargs):
        return create_model(
            architecture="Unet",
            encoder_name=backbone_name,
            encoder_weights="imagenet" if pretrained else None,
            in_channels=3,
            classes=num_classes,
            activation="sigmoid"
        )


if __name__ == "__main__":
    # Test the model
    print("Testing model creation...")
    
    # Test various architectures
    test_architectures = ["Unet", "DeepLabV3Plus", "FPN"]
    
    for arch in test_architectures:
        model = create_model(
            architecture=arch,
            encoder_name="resnet34",
            encoder_weights=None,
            in_channels=3,
            classes=1,
            activation="sigmoid"
        )
        
        x = torch.randn(1, 3, 256, 256)
        y = model(x)
        
        print(f"{arch}:")
        print(f"  Input shape: {x.shape}")
        print(f"  Output shape: {y.shape}")
        print(f"  Total parameters: {sum(p.numel() for p in model.parameters()):,}")
        print()
