from lucas import workflow, function, Workflow
from lucas.serverless_function import Metadata

from lucas.actorc.actor import (
    ActorContext,
    ActorFunction,
    ActorExecutor,
    ActorRuntime,
)
import uuid

import pandas as pd
import torch
from datasets import Dataset
from modelscope import snapshot_download, AutoTokenizer
# 修改导入部分
from modelscope import AutoModelForCausalLM as MS_AutoModelForCausalLM
from transformers import AutoModelForCausalLM, TrainingArguments, Trainer, DataCollatorForSeq2Seq, BitsAndBytesConfig # type: ignore
import os
import sys
from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training, PeftModel
from io import StringIO
from pathlib import Path

context = ActorContext.createContext()

PROMPT = "你是一个电力系统专家，你需要根据用户的问题，给出带有思考的回答。"
MAX_LENGTH = 600

lora_path = "./eps_checkpoint"

tokenizer = AutoTokenizer.from_pretrained("Qwen/Qwen3-0.6B", use_fast=False, trust_remote_code=True)
tokenizer.padding_side = 'left'


@function(
    wrapper=ActorFunction,
    dependency=["--index-url https://download.pytorch.org/whl/cpu torch", "-i https://pypi.mirrors.ustc.edu.cn/simple datasets", "-i https://pypi.mirrors.ustc.edu.cn/simple pandas", "-i https://pypi.mirrors.ustc.edu.cn/simple modelscope", "-i https://pypi.mirrors.ustc.edu.cn/simple transformers"],
    provider="actor",
    name="read_data",
    venv="test2",
    cpu=500,
    memory="1Gi",
    gpu=0,
) # type: ignore
def read_data(content: str):

    def process_func(example):
        """
        将数据集进行预处理
        """ 
        input_ids, attention_mask, labels = [], [], []
        instruction = tokenizer(
            f"<|im_start|>system\n{example['instruction']}<|im_end|>\n<|im_start|>user\n{example['input']}<|im_end|>\n<|im_start|>assistant\n",
            add_special_tokens=False,
        )
        response = tokenizer(f"{example['output']}", add_special_tokens=False)
        input_ids = instruction["input_ids"] + response["input_ids"] + [tokenizer.pad_token_id]
        # 手动添加结束符
        attention_mask = (
            instruction["attention_mask"] + response["attention_mask"] + [1]
        )
        labels = [-100] * len(instruction["input_ids"]) + response["input_ids"] + [tokenizer.pad_token_id]
        if len(input_ids) > MAX_LENGTH:  # 做一个截断
            input_ids = input_ids[:MAX_LENGTH]
            attention_mask = attention_mask[:MAX_LENGTH]
            labels = labels[:MAX_LENGTH]
        return {"input_ids": input_ids, "attention_mask": attention_mask, "labels": labels}   

    train_df = pd.read_json(StringIO(content), lines=True)
    train_ds = Dataset.from_pandas(train_df)
    train_dataset = train_ds.map(process_func, remove_columns=train_ds.column_names)
    return train_dataset


model1 = None
args = None
dc = None
inited = False
CACHE_DIR = "/app/cache"

@function(
    wrapper=ActorFunction,
    dependency=["-i https://pypi.mirrors.ustc.edu.cn/simple datasets", "-i https://pypi.mirrors.ustc.edu.cn/simple modelscope", "--index-url https://download.pytorch.org/whl/cpu torch", "-i https://pypi.mirrors.ustc.edu.cn/simple peft", "-i https://pypi.mirrors.ustc.edu.cn/simple transformers"],
    provider="actor",
    name="train",
    venv="test2",
    cpu=2000,
    memory="5Gi",
    gpu=0,
) # type: ignore
def train(ds: Dataset):
    os.environ['HOME'] = "/app"
    os.environ["MODELSCOPE_HOME"] = CACHE_DIR
    os.environ["MODELSCOPE_CACHE"] = CACHE_DIR
    os.environ["HUGGINGFACE_HUB_CACHE"] = CACHE_DIR
    Path("/app/.cache").mkdir(parents=True, exist_ok=True)
    Path(CACHE_DIR).mkdir(parents=True, exist_ok=True)

    def initOnce():
        global model1, args, dc, inited
        if inited:
            print("llmTest7", file=sys.stderr)
            return
        inited = True
        args = TrainingArguments(
            # output_dir="/home/spark4862/Documents/projects/go/ignis/clients/demo/output/Qwen3-0.6B",
            per_device_train_batch_size=1, # CPU 需要更小的 batch size
            gradient_accumulation_steps=4,
            eval_strategy="no",
            logging_steps=1, # logging可以保留，用于观察loss
            save_steps=100000, # 防止Trainer自动保存
            learning_rate=1e-3,
            save_on_each_node=True,
            gradient_checkpointing=True,
            run_name="qwen3-0.6B-manual-loop",
            optim="adamw_torch", # 移除 bnb 优化器
            use_cpu=True,  # 显式禁用 CUDA
            fp16=False,  # 确保不使用半精度
            bf16=False,  # 确保不使用bfloat16
        )

        dc = DataCollatorForSeq2Seq(tokenizer=tokenizer, padding=True, pad_to_multiple_of=8)

        lora_config = LoraConfig(
            r=8,
            lora_alpha=16,
            target_modules="all-linear",
            lora_dropout=0.05,
            bias="none",
            task_type="CAUSAL_LM",
        )


        print("--- Loading model ---")
        # global model
        model1 = MS_AutoModelForCausalLM.from_pretrained(
            "Qwen/Qwen3-0.6B", 
            dtype=torch.float32, 
            trust_remote_code=True,
            cache_dir=CACHE_DIR,
            use_cache=False,   # 关键：禁用缓存
            # quantization_config=bnb_config,
            # attn_implementation="flash_attention_2"
        )
        print("llmTest0", file=sys.stderr)


        #model1 = prepare_model_for_kbit_training(model1)
        if os.path.exists(lora_path):
            print("llmTest1", file=sys.stderr)
            model1 = PeftModel.from_pretrained(model1, lora_path)
        else:
            print("llmTest2", file=sys.stderr)
            model1 = get_peft_model(model1, lora_config)
        print("llmTest3", file=sys.stderr)
        model1.gradient_checkpointing_enable() # type: ignore
        model1.enable_input_require_grads() # type: ignore

        model1 = model1.to("cpu")
        print("llmTest4", file=sys.stderr)

    initOnce()
    
    print("llmTest5", file=sys.stderr)
    trainer = Trainer(
        model=model1,
        args=args,
        train_dataset=ds,
        data_collator=dc,
    )
    print("llmTest6", file=sys.stderr)
    trainer.train()
    model1.save_pretrained(lora_path)
    return "test"



@workflow(executor=ActorExecutor) # type: ignore
def workflowfunc(wf: Workflow):
    _in = wf.input()

    ds = wf.call("read_data", {"content": _in["content"]})
    wf.call("train", {"ds": ds})
    return "test"

def actorWorkflowExportFunc(dict: dict):

    # just use for local invoke
    from lucas import routeBuilder

    route = routeBuilder.build()
    route_dict = {}
    for function in route.functions:
        route_dict[function.name] = function.handler
    for workflow in route.workflows:
        route_dict[workflow.name] = function.handler
    metadata = Metadata(
        id=str(uuid.uuid4()),
        params=dict,
        namespace=None,
        router=route_dict,
        request_type="invoke",
        redis_db=None,
        producer=None,
    )
    rt = ActorRuntime(metadata)
    workflowfunc.set_runtime(rt)
    workflow = workflowfunc.generate()
    return workflow.execute()


workflow_func = workflowfunc.export(actorWorkflowExportFunc)
print("----first execute----")
data_dir = "/home/xhy/iarnet-demo/eps_group"

for i in range(5):
    for file in os.listdir(data_dir):
        with open(os.path.join(data_dir, file), "r") as f:
            workflow_func({"content": f.read()})

