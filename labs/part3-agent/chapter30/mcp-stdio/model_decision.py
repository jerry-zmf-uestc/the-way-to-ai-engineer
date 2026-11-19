"""One bounded local generation. MCP executes tools in the Go Host, never here."""
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import sys


def main(assets):
    if sys.version_info[:3] != (3, 12, 14):
        raise ValueError('locked Python required')
    for line in Path(__file__).with_name('generation-requirements.lock').read_text().splitlines():
        name, version = line.split('==')
        if importlib.metadata.version(name) != version:
            raise ValueError('locked dependency required')
    raw = sys.stdin.buffer.read((16 << 10)+1)
    if len(raw) > 16 << 10:
        raise ValueError('input byte budget')
    view = json.loads(raw)
    if set(view) not in ({'Tools','History'},{'Tools','History','CallID'}) or not isinstance(view['Tools'],list) or len(view['Tools'])>32 or not isinstance(view['History'],list) or len(view['History'])>16:
        raise ValueError('input shape')
    if 'CallID' in view and (not isinstance(view['CallID'],str) or not 0<len(view['CallID'])<=128):raise ValueError('call binding shape')
    lock = json.loads(Path(__file__).with_name('model-lock.json').read_text())
    root = Path(assets).resolve()
    for item in lock['files']:
        path = root/item['path']
        if path.is_symlink() or path.stat().st_size != item['bytes']:
            raise ValueError('asset identity')
        digest = hashlib.sha256()
        with path.open('rb') as stream:
            for chunk in iter(lambda:stream.read(1<<20),b''):
                digest.update(chunk)
        if digest.hexdigest() != item['sha256']:
            raise ValueError('asset checksum')
    os.environ['HF_HUB_OFFLINE']=os.environ['TRANSFORMERS_OFFLINE']='1'
    import torch
    from transformers import AutoTokenizer,AutoModelForCausalLM
    torch.set_num_threads(1)
    tokenizer=AutoTokenizer.from_pretrained(str(root),local_files_only=True,trust_remote_code=False)
    model=AutoModelForCausalLM.from_pretrained(str(root),local_files_only=True,trust_remote_code=False,use_safetensors=True).to('cpu').eval()
    instruction=('Choose the next action for the user question. Return only one JSON object: '
                 '{"Call":{"ID":"a unique call id","Tool":"one exact Tools name","DocumentID":"doc-A"}} '
                 'to read the required document, or {"Final":"answer supported by tool observations"} to finish. '
                 'Read requested sources before comparing. Never invent tool results. Call IDs must not repeat. '
                 'Do not combine Final and Call. No markdown fences or explanation outside JSON.')
    if 'CallID' in view:
        instruction=('Choose the next action for the user question. Return only one JSON object: '
                     '{"Call":{"Tool":"one exact Tools name","DocumentID":"doc-A"}} to read, '
                     'or {"Final":"answer supported by observed tool results"} to finish. '
                     'Do not emit ID: the application binds Call to the supplied CallID. '
                     'Use observations to determine which requested source still needs reading. '
                     'Never invent tool results. Do not combine Final and Call. No markdown or explanation outside JSON.')
    prompt=tokenizer.apply_chat_template([{'role':'system','content':instruction},{'role':'user','content':json.dumps(view)}],
                                         tokenize=False,add_generation_prompt=True,enable_thinking=False)
    inputs=tokenizer(prompt,return_tensors='pt',add_special_tokens=False)
    count=inputs['input_ids'].shape[1]
    if count>1024:raise ValueError('input token budget')
    with torch.inference_mode():
        reply=model.generate(**inputs,do_sample=False,max_new_tokens=192,use_cache=True,pad_token_id=tokenizer.eos_token_id)
    suffix=reply[0,count:]
    print(json.dumps({'Raw':tokenizer.decode(suffix,skip_special_tokens=True),'InputTokens':count,
                      'OutputTokens':len(suffix),'AtOutputLimit':len(suffix)==192}))


if __name__=='__main__':
    if os.environ.get('MCP_MODEL_E2E')!='1':sys.exit('explicit MCP_MODEL_E2E=1 required')
    parser=argparse.ArgumentParser();parser.add_argument('--assets',required=True)
    main(parser.parse_args().assets)
