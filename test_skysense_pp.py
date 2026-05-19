#!/usr/bin/env python3
"""
Test script for SkySense++ model integration.
"""

import sys
import torch

def test_skysense_pp_model():
    """Test SkySense++ model creation and forward pass."""
    print("=" * 60)
    print("Testing SkySense++ Model Integration")
    print("=" * 60)
    
    try:
        from models.skysense_pp import SkySensePPModel, create_skysense_pp_model_from_config
        
        print("\n1. Testing direct SkySensePPModel creation...")
        model = SkySensePPModel(
            encoder_name='resnet34',
            in_channels=4,
            num_classes=1,
            fusion_type='adaptive',
            use_semantic_enhancement=True
        )
        
        x = torch.randn(2, 4, 256, 256)
        y = model(x)
        
        print(f"   ✓ Input shape: {x.shape}")
        print(f"   ✓ Output shape: {y.shape}")
        print(f"   ✓ Total parameters: {sum(p.numel() for p in model.parameters()):,}")
        
        print("\n2. Testing with transformer fusion...")
        model2 = SkySensePPModel(
            encoder_name='resnet34',
            in_channels=4,
            num_classes=1,
            fusion_type='transformer',
            use_semantic_enhancement=True
        )
        
        y2 = model2(x)
        print(f"   ✓ Transformer fusion output shape: {y2.shape}")
        
        print("\n3. Testing without semantic enhancement...")
        model3 = SkySensePPModel(
            encoder_name='resnet34',
            in_channels=4,
            num_classes=1,
            fusion_type='adaptive',
            use_semantic_enhancement=False
        )
        
        y3 = model3(x)
        print(f"   ✓ Without semantic enhancement output shape: {y3.shape}")
        
        print("\n4. Testing with config dict...")
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
        
        model4 = create_skysense_pp_model_from_config(test_config)
        y4 = model4(x)
        print(f"   ✓ Config-based model output shape: {y4.shape}")
        
        print("\n5. Testing model compatibility with existing codebase...")
        from models.unet import create_model_from_config
        
        class MockConfig:
            def __init__(self, cfg_dict):
                self.model = cfg_dict
        
        config = MockConfig(test_config['model'])
        model5 = create_model_from_config(config)
        y5 = model5(x)
        print(f"   ✓ create_model_from_config compatibility: {y5.shape}")
        
        print("\n" + "=" * 60)
        print("All SkySense++ integration tests passed! ✓")
        print("=" * 60)
        
        return True
        
    except Exception as e:
        print(f"\n❌ Error during testing: {e}")
        import traceback
        traceback.print_exc()
        return False


def test_backward_compatibility():
    """Test backward compatibility with existing models."""
    print("\n" + "=" * 60)
    print("Testing Backward Compatibility")
    print("=" * 60)
    
    try:
        from models.unet import create_model
        
        architectures = ['Unet', 'UnetPlusPlus', 'DeepLabV3Plus', 'FPN']
        
        for arch in architectures:
            print(f"\nTesting {arch}...")
            model = create_model(
                architecture=arch,
                encoder_name='resnet34',
                encoder_weights=None,
                in_channels=3,
                classes=1,
                activation='sigmoid'
            )
            
            x = torch.randn(2, 3, 256, 256)
            y = model(x)
            
            print(f"   ✓ {arch}: Input {x.shape} -> Output {y.shape}")
        
        print("\n" + "=" * 60)
        print("Backward compatibility tests passed! ✓")
        print("=" * 60)
        
        return True
        
    except Exception as e:
        print(f"\n❌ Error during backward compatibility test: {e}")
        import traceback
        traceback.print_exc()
        return False


if __name__ == '__main__':
    success1 = test_skysense_pp_model()
    success2 = test_backward_compatibility()
    
    if success1 and success2:
        print("\n🎉 All tests passed successfully!")
        sys.exit(0)
    else:
        print("\n⚠️ Some tests failed.")
        sys.exit(1)
