#!/usr/bin/env python3
"""
Download 38-Cloud dataset using kagglehub
"""

import kagglehub
import os
import sys

def download_38cloud_dataset():
    """Download the 38-Cloud dataset."""
    print("=" * 60)
    print("Downloading 38-Cloud Cloud Segmentation Dataset")
    print("=" * 60)
    
    try:
        # Download latest version
        path = kagglehub.dataset_download("sorour/38cloud-cloud-segmentation-in-satellite-images")
        
        print(f"\n✓ Dataset downloaded successfully!")
        print(f"Path: {path}")
        
        # List contents
        print(f"\nDataset contents:")
        for item in os.listdir(path):
            item_path = os.path.join(path, item)
            if os.path.isdir(item_path):
                print(f"  📁 {item}/")
                sub_items = os.listdir(item_path)[:5]
                for sub_item in sub_items:
                    print(f"      - {sub_item}")
                if len(os.listdir(item_path)) > 5:
                    print(f"      ... and {len(os.listdir(item_path)) - 5} more items")
            else:
                print(f"  📄 {item}")
        
        return path
        
    except Exception as e:
        print(f"\n✗ Error downloading dataset: {e}")
        import traceback
        traceback.print_exc()
        return None

if __name__ == '__main__':
    path = download_38cloud_dataset()
    if path:
        print(f"\n{'=' * 60}")
        print(f"Next step: Update config/config.yaml with path: {path}")
        print(f"{'=' * 60}")
