"""
Inference script for cloud segmentation
"""

import argparse
import torch
import cv2
import numpy as np
from pathlib import Path
import matplotlib.pyplot as plt

from models.unet import create_model


def load_image(image_path, size=256):
    """Load and preprocess image"""
    
    image = cv2.imread(image_path)
    
    if image is None:
        raise ValueError(f"Could not load image: {image_path}")
    
    # Convert BGR to RGB
    image = cv2.cvtColor(image, cv2.COLOR_BGR2RGB)
    
    # Resize
    original_size = image.shape[:2]
    image = cv2.resize(image, (size, size))
    
    # Normalize
    image = image.astype(np.float32) / 255.0
    
    # Convert to tensor
    image = torch.from_numpy(image).permute(2, 0, 1).unsqueeze(0)
    
    return image, original_size


def predict(model, image_tensor, device):
    """Make prediction"""
    
    model.eval()
    
    with torch.no_grad():
        image_tensor = image_tensor.to(device)
        output = model(image_tensor)
    
    # Convert to numpy
    prediction = output.squeeze().cpu().numpy()
    prediction = (prediction > 0.5).astype(np.uint8) * 255
    
    return prediction


def visualize_results(original_image, prediction, output_path=None):
    """Visualize original image and prediction"""
    
    fig, axes = plt.subplots(1, 2, figsize=(12, 5))
    
    # Original image
    axes[0].imshow(original_image)
    axes[0].set_title('Original Image')
    axes[0].axis('off')
    
    # Prediction
    axes[1].imshow(prediction, cmap='gray')
    axes[1].set_title('Cloud Segmentation')
    axes[1].axis('off')
    
    plt.tight_layout()
    
    if output_path:
        plt.savefig(output_path, dpi=150, bbox_inches='tight')
        print(f"✓ Visualization saved: {output_path}")
    
    plt.show()


def create_overlay(original_image, prediction):
    """Create overlay of prediction on original image"""
    
    # Convert to uint8
    original = (original_image * 255).astype(np.uint8)
    
    # Create colored overlay
    overlay = original.copy()
    cloud_pixels = prediction > 128
    overlay[cloud_pixels] = [255, 0, 0]  # Red for clouds
    
    # Blend
    result = cv2.addWeighted(original, 0.7, overlay, 0.3, 0)
    
    return result


def main(image_path, model_path, output_dir=None, device_name='cuda', visualize=True, 
          architecture='Unet', encoder_name='resnet34'):
    """Main inference function"""
    
    # Device
    device = torch.device(device_name if torch.cuda.is_available() else 'cpu')
    print(f"Using device: {device}\n")
    
    # Load model using segmentation_models_pytorch
    print(f"Loading model from: {model_path}")
    
    model = create_model(
        architecture=architecture,
        encoder_name=encoder_name,
        encoder_weights=None,
        in_channels=3,
        classes=1,
        activation='sigmoid'
    )
    
    if not Path(model_path).exists():
        print(f"Error: Model file not found: {model_path}")
        return
    
    model.load_state_dict(torch.load(model_path, map_location=device))
    model = model.to(device)
    
    total_params = sum(p.numel() for p in model.parameters())
    print(f"Model loaded with {total_params:,} parameters\n")
    
    # Load image
    print(f"Loading image from: {image_path}")
    image_tensor, original_size = load_image(image_path)
    print(f"Image size: {original_size}\n")
    
    # Predict
    print("Making prediction...")
    prediction = predict(model, image_tensor, device)
    
    # Resize back to original size
    prediction = cv2.resize(prediction, (original_size[1], original_size[0]))
    
    # Load original image for visualization
    original_image = cv2.imread(image_path)
    original_image = cv2.cvtColor(original_image, cv2.COLOR_BGR2RGB)
    original_image = original_image.astype(np.float32) / 255.0
    
    # Save results
    if output_dir:
        output_dir = Path(output_dir)
        output_dir.mkdir(parents=True, exist_ok=True)
        
        # Save prediction mask
        mask_path = output_dir / f"{Path(image_path).stem}_mask.png"
        cv2.imwrite(str(mask_path), prediction)
        print(f"✓ Mask saved: {mask_path}")
        
        # Save overlay
        overlay = create_overlay(original_image, prediction)
        overlay_path = output_dir / f"{Path(image_path).stem}_overlay.png"
        cv2.imwrite(str(overlay_path), cv2.cvtColor(overlay, cv2.COLOR_RGB2BGR))
        print(f"✓ Overlay saved: {overlay_path}")
    
    # Visualize
    if visualize:
        visualize_results(original_image, prediction)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Predict cloud segmentation')
    parser.add_argument('--image_path', type=str, required=True,
                       help='Path to input image')
    parser.add_argument('--model_path', type=str, required=True,
                       help='Path to model checkpoint')
    parser.add_argument('--output_dir', type=str, default=None,
                       help='Output directory for results')
    parser.add_argument('--device', type=str, default='cuda',
                       choices=['cuda', 'cpu'], help='Device to use')
    parser.add_argument('--visualize', action='store_true',
                       help='Visualize results')
    parser.add_argument('--architecture', type=str, default='Unet',
                       choices=['Unet', 'UnetPlusPlus', 'DeepLabV3', 'DeepLabV3Plus', 
                                'FPN', 'PSPNet', 'Linknet', 'MAnet', 'PAN'],
                       help='Model architecture')
    parser.add_argument('--encoder_name', type=str, default='resnet34',
                       help='Encoder backbone name (e.g., resnet18, resnet34, resnet50, efficientnet-b0, etc.)')
    
    args = parser.parse_args()
    
    main(args.image_path, args.model_path, args.output_dir, args.device, 
         args.visualize, args.architecture, args.encoder_name)
