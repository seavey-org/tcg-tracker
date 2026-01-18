#!/usr/bin/env python3
"""
Train YOLOv8-Nano Segmentation model for card localization.

This script trains a YOLOv8-Nano instance segmentation model on synthetic
card images to detect and segment trading cards in photos.

Usage:
    # Train on synthetic dataset
    python -m identifier.scripts.train_yolo_detector \
        --data-yaml ./data/synthetic_dataset/data.yaml \
        --epochs 100 \
        --output-dir ./models/card_detector

    # Resume training
    python -m identifier.scripts.train_yolo_detector \
        --resume ./models/card_detector/weights/last.pt

    # Export to ONNX
    python -m identifier.scripts.train_yolo_detector --export-onnx ./models/card_detector.onnx

Requirements:
    pip install ultralytics
"""

import argparse
import logging
import sys
from pathlib import Path

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)


def train_model(
    data_yaml: str,
    epochs: int = 100,
    batch_size: int = 16,
    img_size: int = 640,
    output_dir: str = "./models/card_detector",
    model_size: str = "n",  # n=nano, s=small, m=medium, l=large, x=xlarge
    resume: str | None = None,
    device: str | None = None,
) -> str:
    """Train YOLOv8 segmentation model.

    Args:
        data_yaml: Path to dataset YAML config
        epochs: Number of training epochs
        batch_size: Training batch size
        img_size: Input image size
        output_dir: Output directory for trained model
        model_size: Model size (n/s/m/l/x)
        resume: Path to checkpoint to resume from
        device: Device to train on (None for auto, "cpu", "0", "0,1", etc.)

    Returns:
        Path to best model weights
    """
    try:
        from ultralytics import YOLO
    except ImportError:
        logger.error("ultralytics not installed. Install with: pip install ultralytics")
        raise

    output_path = Path(output_dir)
    output_path.mkdir(parents=True, exist_ok=True)

    if resume:
        logger.info(f"Resuming training from {resume}")
        model = YOLO(resume)
    else:
        # Load pretrained YOLOv8-seg model
        model_name = f"yolov8{model_size}-seg.pt"
        logger.info(f"Loading pretrained {model_name}")
        model = YOLO(model_name)

    # Train the model
    logger.info(f"Starting training for {epochs} epochs")
    logger.info(f"  Dataset: {data_yaml}")
    logger.info(f"  Batch size: {batch_size}")
    logger.info(f"  Image size: {img_size}")
    logger.info(f"  Output: {output_dir}")

    model.train(
        data=data_yaml,
        epochs=epochs,
        batch=batch_size,
        imgsz=img_size,
        project=str(output_path.parent),
        name=output_path.name,
        device=device,
        exist_ok=True,
        # Augmentation settings
        augment=True,
        hsv_h=0.015,
        hsv_s=0.7,
        hsv_v=0.4,
        degrees=45.0,
        translate=0.1,
        scale=0.5,
        shear=5.0,
        perspective=0.001,
        flipud=0.0,
        fliplr=0.5,
        mosaic=1.0,
        mixup=0.1,
        # Training settings
        patience=20,
        close_mosaic=10,
        optimizer="AdamW",
        lr0=0.01,
        lrf=0.01,
        momentum=0.937,
        weight_decay=0.0005,
        warmup_epochs=3,
        warmup_momentum=0.8,
        warmup_bias_lr=0.1,
        # Save settings
        save=True,
        save_period=10,
        # Logging
        verbose=True,
        plots=True,
    )

    best_model_path = str(output_path / "weights" / "best.pt")
    logger.info(f"Training complete. Best model saved to: {best_model_path}")

    return best_model_path


def validate_model(model_path: str, data_yaml: str) -> dict:
    """Validate trained model on validation set.

    Args:
        model_path: Path to model weights
        data_yaml: Path to dataset YAML config

    Returns:
        Validation metrics
    """
    from ultralytics import YOLO

    logger.info(f"Validating model: {model_path}")
    model = YOLO(model_path)

    results = model.val(data=data_yaml)

    metrics = {
        "mAP50": results.seg.map50,
        "mAP50-95": results.seg.map,
        "precision": results.seg.mp,
        "recall": results.seg.mr,
    }

    logger.info("Validation results:")
    for k, v in metrics.items():
        logger.info(f"  {k}: {v:.4f}")

    return metrics


def export_model(
    model_path: str,
    format: str = "onnx",
    output_path: str | None = None,
    img_size: int = 640,
) -> str:
    """Export model to different formats.

    Args:
        model_path: Path to model weights
        format: Export format (onnx, torchscript, openvino, etc.)
        output_path: Optional output path
        img_size: Input image size for export

    Returns:
        Path to exported model
    """
    from ultralytics import YOLO

    logger.info(f"Exporting model to {format} format")
    model = YOLO(model_path)

    exported_path = model.export(
        format=format,
        imgsz=img_size,
        simplify=True if format == "onnx" else False,
        dynamic=False,
    )

    if output_path:
        import shutil
        shutil.move(str(exported_path), output_path)
        exported_path = output_path

    logger.info(f"Model exported to: {exported_path}")
    return str(exported_path)


def predict_image(model_path: str, image_path: str, output_path: str | None = None) -> list:
    """Run inference on a single image.

    Args:
        model_path: Path to model weights
        image_path: Path to input image
        output_path: Optional path to save annotated image

    Returns:
        List of detections
    """
    from ultralytics import YOLO

    model = YOLO(model_path)
    results = model(image_path)

    detections = []
    for r in results:
        for i, (box, mask) in enumerate(zip(r.boxes, r.masks.xy if r.masks else [])):
            det = {
                "confidence": float(box.conf),
                "class": int(box.cls),
                "class_name": r.names[int(box.cls)],
                "bbox": box.xyxy[0].tolist(),
                "polygon": mask.tolist() if len(mask) > 0 else [],
            }
            detections.append(det)

    if output_path:
        annotated = results[0].plot()
        import cv2
        cv2.imwrite(output_path, annotated)
        logger.info(f"Annotated image saved to: {output_path}")

    return detections


def main():
    parser = argparse.ArgumentParser(
        description="Train YOLOv8-Nano Segmentation model for card detection"
    )
    subparsers = parser.add_subparsers(dest="command", help="Commands")

    # Train command
    train_parser = subparsers.add_parser("train", help="Train model")
    train_parser.add_argument(
        "--data-yaml",
        type=str,
        required=True,
        help="Path to dataset YAML config",
    )
    train_parser.add_argument(
        "--epochs",
        type=int,
        default=100,
        help="Number of training epochs (default: 100)",
    )
    train_parser.add_argument(
        "--batch-size",
        type=int,
        default=16,
        help="Training batch size (default: 16)",
    )
    train_parser.add_argument(
        "--img-size",
        type=int,
        default=640,
        help="Input image size (default: 640)",
    )
    train_parser.add_argument(
        "--output-dir",
        type=str,
        default="./models/card_detector",
        help="Output directory for trained model",
    )
    train_parser.add_argument(
        "--model-size",
        type=str,
        default="n",
        choices=["n", "s", "m", "l", "x"],
        help="Model size: n=nano, s=small, m=medium, l=large, x=xlarge (default: n)",
    )
    train_parser.add_argument(
        "--resume",
        type=str,
        help="Path to checkpoint to resume from",
    )
    train_parser.add_argument(
        "--device",
        type=str,
        help="Device to train on (auto, cpu, 0, 0,1, etc.)",
    )

    # Validate command
    val_parser = subparsers.add_parser("validate", help="Validate model")
    val_parser.add_argument("--model", type=str, required=True, help="Model weights path")
    val_parser.add_argument("--data-yaml", type=str, required=True, help="Dataset YAML config")

    # Export command
    export_parser = subparsers.add_parser("export", help="Export model")
    export_parser.add_argument("--model", type=str, required=True, help="Model weights path")
    export_parser.add_argument(
        "--format",
        type=str,
        default="onnx",
        choices=["onnx", "torchscript", "openvino", "coreml", "tflite"],
        help="Export format (default: onnx)",
    )
    export_parser.add_argument("--output", type=str, help="Output path")
    export_parser.add_argument("--img-size", type=int, default=640, help="Image size for export")

    # Predict command
    predict_parser = subparsers.add_parser("predict", help="Run inference on image")
    predict_parser.add_argument("--model", type=str, required=True, help="Model weights path")
    predict_parser.add_argument("--image", type=str, required=True, help="Input image path")
    predict_parser.add_argument("--output", type=str, help="Output annotated image path")

    args = parser.parse_args()

    if args.command == "train":
        train_model(
            data_yaml=args.data_yaml,
            epochs=args.epochs,
            batch_size=args.batch_size,
            img_size=args.img_size,
            output_dir=args.output_dir,
            model_size=args.model_size,
            resume=args.resume,
            device=args.device,
        )
    elif args.command == "validate":
        validate_model(args.model, args.data_yaml)
    elif args.command == "export":
        export_model(args.model, args.format, args.output, args.img_size)
    elif args.command == "predict":
        detections = predict_image(args.model, args.image, args.output)
        import json
        print(json.dumps(detections, indent=2))
    else:
        parser.print_help()
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
