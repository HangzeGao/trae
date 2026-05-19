#!/usr/bin/env python3
"""
Download 38-Cloud dataset using kagglehub.
"""

try:
    import kagglehub
    print("Downloading 38-Cloud dataset...")
    path1 = kagglehub.dataset_download("sorour/38cloud-cloud-segmentation-in-satellite-images")
    print(f"38-Cloud dataset downloaded to: {path1}")
    print("\nDownloading 95-Cloud dataset...")
    path2 = kagglehub.dataset_download("sorour/95cloud-cloud-segmentation-on-satellite-images")
    print(f"95-Cloud dataset downloaded to: {path2}")
    
    print("\nDatasets downloaded successfully!")
    
except ImportError:
    print("Error: kagglehub is not installed.")
    print("Please install with: pip install kagglehub")
    import sys
    sys.exit(1)
except Exception as e:
    print(f"Error downloading datasets: {e}")
    import sys
    sys.exit(1)
