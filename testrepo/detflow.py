import os
import uuid
from lucas import workflow, function, Workflow
from lucas.serverless_function import Metadata
from lucas.actorc.actor import (
    ActorContext,
    ActorFunction,
    ActorExecutor,
    ActorRuntime,
)
from lucas.utils.logging import log

# 创建Actor上下文
context = ActorContext.createContext()


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="collect_image",
    backend="go",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=1,
)
def collect_image(Image):
    r"""
package main

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
)

const Name = "CollectImage"

type Input struct {
	Image string
}

type Output = image.Image

func Impl(input *Input) (Output, error) {
	data, err := os.ReadFile(input.Image)
	if err != nil {
		return nil, err
	}

	im, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	log.Printf("%s: read image %dx%d, format %s\n", Name, im.Bounds().Dx(), im.Bounds().Dy(), format)
	return im, nil
}
    """
    pass  # Go代码在docstring中


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="central_crop",
    backend="go",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=1,
)
def central_crop(Image):
    """
package main

import (
	"image"
	"log"

	"github.com/disintegration/imaging"
)

const Name = "CentralCrop"

type Input struct {
	Image image.Image
}

type Output = image.Image

func Impl(input *Input) (Output, error) {
	im := input.Image

	width := min(im.Bounds().Dx(), im.Bounds().Dy())
	squared := imaging.Fill(im, width, width, imaging.Center, imaging.Lanczos)

	log.Printf("%s: %dx%d", Name, width, width)

	return squared, nil
}
    """
    pass  # Go代码在docstring中


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="resize",
    backend="go",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=1,
)
def resize(Image):
    """
package main

import (
	"bytes"
	"image"
	"log"

	"github.com/disintegration/imaging"
)

const Name = "ResizeImage"

type Input struct {
	Image   image.Image
}

type Output = []byte

func Impl(input *Input) (Output, error) {
    width, height := 1280, 1280
	resized := imaging.Resize(input.Image, width, height, imaging.BSpline)

	log.Printf("%s: %dx%d", Name, width, height)

	buf := bytes.NewBuffer(nil)
	err := imaging.Encode(buf, resized, imaging.JPEG)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
    """
    pass  # Go代码在docstring中

@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="alert",
    backend="unikernel",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=1,
)
def alert(result):
    """
open Yojson.Basic.Util

let alert value =
  try
    let data = value |> member "result" in
    Ok (`String "ok")
  with Type_error (msg, _) -> Error ("input type error: " ^ msg)

let handlers = [
  "alert", alert;
]
"""
    pass  # OCaml代码在docstring中


import os

import cv2
import numpy as np
import torch
from lucas import workflow, function, Workflow
from lucas.serverless_function import Metadata
from lucas.actorc.actor import (
    ActorContext,
    ActorFunction,
    ActorExecutor,
    ActorRuntime,
)
import uuid

# from torchvision.models.detection import (
#     ssdlite320_mobilenet_v3_large,
#     SSDLite320_MobileNet_V3_Large_Weights,
# )
# from torchvision.models.detection.ssd import SSDHead
# from torchvision.transforms import functional as F

from torchvision.transforms import functional as F

from ultralytics import YOLO

# device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
device = "cpu"


@function(
    wrapper=ActorFunction,
    dependency=["--index-url https://download.pytorch.org/whl/cpu torch", "--index-url https://download.pytorch.org/whl/cpu torchvision", "numpy"],
    provider="actor",
    name="inference",
    venv="test2",
    cpu=3000,  # millicores
    memory="3Gi",
    gpu=0,
    replicas=1,
)
def inference(im: np.ndarray):
    def load_helmet_model():
    # 默认 GPU 版权重（用于有 GPU 的环境）
        gpu_model_path = "/app/eps_helmet/best_cpu.pt"
        # CPU 版权重（我们在 helmet_model_checkpoints 中已用脚本转换生成）
        cpu_model_path = "/app/eps_helmet/best_cpu.pt"

        # 根据当前设备优先选择合适的权重
        if torch.cuda.is_available() and os.path.exists(gpu_model_path):
            model_path = gpu_model_path
        elif os.path.exists(cpu_model_path):
            # 在纯 CPU 环境下优先使用 CPU 版权重，避免 CUDA 反序列化错误
            model_path = cpu_model_path
        else:
            # 兜底：如果以上都不存在，仍尝试原来的 GPU 版路径（可能会在 CPU 上报错）
            model_path = gpu_model_path
        
        print("正在加载 YOLO 模型...")
        print(f"  - 选择的权重文件: {model_path}")
        model = YOLO(model_path)
        
        if torch.cuda.is_available():
            model.to(device)
        
        print("✓ 已加载训练好的头盔检测模型")
        print(f"  - 设备: {device}")
        
        return model

    model = load_helmet_model()

    image_tensor = F.to_tensor(im).unsqueeze(0)

    with torch.no_grad():
        predictions = model(image_tensor)

    return predictions[0]


@function(
    wrapper=ActorFunction,
    dependency=["--index-url https://download.pytorch.org/whl/cpu torch", "opencv-python-headless", "numpy"],
    provider="actor",
    name="paint",
    venv="test2",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=1,
)
def paint(image: np.ndarray, result: dict):
    confidence_threshold = 0.7  # 置信度阈值
    
    # 现在模型只输出2个类别：0=背景，1=头盔
    helmet_detections = []
    for box, label, score in zip(result["boxes"], result["labels"], result["scores"]):
        if score < confidence_threshold:
            continue
        
        # 只保留label=1的检测结果（头盔类别）
        if label == 1:  # 类别1是头盔
            helmet_detections.append((box, label, score))
    
    # 绘制头盔检测结果
    for box, label, score in helmet_detections:
        x1, y1, x2, y2 = box
        
        # 使用红色框绘制头盔检测结果
        cv2.rectangle(image, (int(x1), int(y1)), (int(x2), int(y2)), (0, 0, 255), 2)
        
        # 使用hard_hat作为类别名称
        class_name = "hard_hat"
        
        # 绘制标签和置信度
        label_text = f"{class_name}: {score:.2f}"
        cv2.putText(
            image,
            label_text,
            (int(x1), int(y1) - 10),
            cv2.FONT_HERSHEY_SIMPLEX,
            0.6,
            (0, 0, 255),
            2,
        )
    
    print(f"检测到头盔数量: {len(helmet_detections)}")
    cv2.imwrite(f"./helmet_detection_result.jpg", image)
    return image

@workflow(executor=ActorExecutor, provider="actor")
def workflowfunc(wf: Workflow):
    _in = wf.input()

    im = wf.call("collect_image", {"Image": _in["image"]}) 
    im = wf.call("central_crop", {"Image": im})
    im = wf.call("resize", {"Image": im})

    pred = wf.call("inference", {"im": im})
    vis = wf.call("paint", {"image": im, "result": pred})
    wf.call("alert", {"result": pred})

    return vis
    # return "ok"

detect = workflowfunc.export()
detect({"image": "/home/zhangyx/iarnet/testrepo/eps_images/000011.jpg"})