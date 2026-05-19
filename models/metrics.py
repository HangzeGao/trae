
import torch


class SegmentationMetrics:
    """Class for segmentation evaluation metrics"""
    
    @staticmethod
    def dice_score(preds, targets, smooth=1e-7):
        """
        Calculate Dice score coefficient
        Args:
            preds: Model predictions (batch, 1, H, W)
            targets: Ground truth masks (batch, 1, H, W)
            smooth: Smoothing factor
        Returns:
            Dice score (float)
        """
        # 确保 preds 在 [0,1] 范围内（应用 sigmoid）
        preds = torch.sigmoid(preds)
        
        # 二值化预测
        preds_binary = (preds > 0.5).float()
        
        # 展平
        preds_flat = preds_binary.contiguous().view(-1)
        targets_flat = targets.contiguous().view(-1)
        
        # 计算交集和并集
        intersection = (preds_flat * targets_flat).sum()
        dice = (2. * intersection + smooth) / (preds_flat.sum() + targets_flat.sum() + smooth)
        
        return dice.item()
    
    @staticmethod
    def iou_score(preds, targets, smooth=1e-7):
        """
        Calculate IoU (Jaccard index)
        Args:
            preds: Model predictions (batch, 1, H, W)
            targets: Ground truth masks (batch, 1, H, W)
            smooth: Smoothing factor
        Returns:
            IoU score (float)
        """
        # 确保 preds 在 [0,1] 范围内
        preds = torch.sigmoid(preds)
        
        # 二值化预测
        preds_binary = (preds > 0.5).float()
        
        # 展平
        preds_flat = preds_binary.contiguous().view(-1)
        targets_flat = targets.contiguous().view(-1)
        
        # 计算交集和并集
        intersection = (preds_flat * targets_flat).sum()
        union = preds_flat.sum() + targets_flat.sum() - intersection
        
        iou = (intersection + smooth) / (union + smooth)
        return iou.item()

