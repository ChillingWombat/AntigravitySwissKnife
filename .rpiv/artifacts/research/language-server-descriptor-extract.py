import sys
data=open('/opt/Antigravity/resources/bin/language_server','rb').read()

def rv(b,i):
    r=0;s=0
    while True:
        x=b[i]; i+=1; r|=(x&0x7f)<<s; s+=7
        if not x&0x80: return r,i

def read_fields(b,i=0,end=None):
    """yield (fieldno, wiretype, value, next_index) tolerant"""
    n=len(b) if end is None else end
    while i<n:
        try: k,i=rv(b,i)
        except IndexError: return
        f=k>>3; w=k&7
        if f==0: return
        try:
            if w==0:
                v,i=rv(b,i); yield f,w,v,i
            elif w==2:
                ln,i=rv(b,i)
                if i+ln>n: return
                yield f,w,b[i:i+ln],i+ln; i+=ln
            elif w==5:
                yield f,w,b[i:i+4],i+4; i+=4
            elif w==1:
                yield f,w,b[i:i+8],i+8; i+=8
            else: return
        except IndexError: return

TYPES={1:'double',2:'float',3:'int64',4:'uint64',5:'int32',6:'fixed64',7:'fixed32',8:'bool',9:'string',10:'group',11:'message',12:'bytes',13:'uint32',14:'enum',15:'sfixed32',16:'sfixed64',17:'sint32',18:'sint64'}
LABELS={1:'optional',2:'required',3:'repeated'}

def pf(b):
    d={}
    for f,w,v,_ in read_fields(b):
        if w!=2 and f!=3 and f!=4 and f!=5 and f!=9 and f!=17: continue
        if f==1: d['name']=v.decode('utf8','replace')
        elif f==3: d['number']=v
        elif f==4: d['label']=v
        elif f==5: d['type']=v
        elif f==6: d['type_name']=v.decode('utf8','replace')
        elif f==10: d['json']=v.decode('utf8','replace')
        elif f==9: d['oneof']=v
        elif f==17: d['p3opt']=v
    return d

def pm(b):
    d={'field':[],'nested':[]}
    for f,w,v,_ in read_fields(b):
        if w!=2: continue
        if f==1: d['name']=v.decode('utf8','replace')
        elif f==2: d['field'].append(pf(v))
        elif f==3: d['nested'].append(pm(v))
    return d

def pmeth(b):
    d={}
    for f,w,v,_ in read_fields(b):
        if f in (1,2,3) and w!=2: continue
        if f==1: d['name']=v.decode('utf8','replace')
        elif f==2: d['input']=v.decode('utf8','replace')
        elif f==3: d['output']=v.decode('utf8','replace')
        elif f==5: d['cs']=v
        elif f==6: d['ss']=v
    return d

def psvc(b):
    d={'method':[]}
    for f,w,v,_ in read_fields(b):
        if w!=2: continue
        if f==1: d['name']=v.decode('utf8','replace')
        elif f==2: d['method'].append(pmeth(v))
    return d

def pfile(b):
    d={'dep':[],'msg':[],'svc':[],'enum':[]}
    for f,w,v,i in read_fields(b):
        if w!=2 and f not in (2,): continue
        if f==1:
            if 'name' in d: break   # end of this descriptor
            d['name']=v.decode('utf8','replace')
        elif f==2: d['package']=v.decode('utf8','replace')
        elif f==3: d['dep'].append(v.decode('utf8','replace'))
        elif f==4: d['msg'].append(pm(v))
        elif f==5: d['enum'].append(v.decode('utf8','replace'))
        elif f==6: d['svc'].append(psvc(v))
    return d

def locate(name):
    nb=name.encode(); out=[]; i=0
    while True:
        o=data.find(nb,i)
        if o<0: return out
        i=o+1
        for k in (1,2,3):
            if o-k<0: continue
            try: ln,j=rv(data,o-k)
            except Exception: continue
            if j==o and ln==len(nb) and data[o-k-1]==0x0a: out.append(o-k-1)

def getfile(name):
    for s in locate(name):
        d=pfile(data[s:s+3000000])
        if d.get('name')==name: return d
    return None

def emit_msg(m,ind=0,out=None):
    p='  '*ind
    out.append(f"{p}message {m['name']} {{")
    for x in m['field']:
        t=TYPES.get(x.get('type'),'?')
        if x.get('type_name'): t=x['type_name'].split('.')[-1]
        lab=LABELS.get(x.get('label'),'')
        if x.get('p3opt'): lab='optional'
        j=x.get('json'); js=f' [json={j}]' if j and j!=x['name'] else ''
        out.append(f"{p}  {lab} {t} {x['name']}={x.get('number')}{js};")
    for n in m['nested']: emit_msg(n,ind+1,out)
    out.append(f"{p}}}")

if __name__=='__main__':
    d=getfile(sys.argv[1])
    if not d: print("NOT FOUND"); sys.exit(1)
    mode=sys.argv[2]
    print(f"# FILE {d['name']}  package={d.get('package')}")
    if mode in ('svc','all'):
        for s in d['svc']:
            print(f"service {s['name']} {{")
            for m in s['method']:
                cs='stream ' if m.get('cs') else ''
                ss='stream ' if m.get('ss') else ''
                print(f"  rpc {m['name']}({cs}{m['input'].split('.')[-1]}) returns ({ss}{m['output'].split('.')[-1]});")
            print("}")
    if mode in ('msg','all'):
        want=set(sys.argv[3:])
        out=[]
        for m in d['msg']:
            if want and m['name'] not in want: continue
            emit_msg(m,0,out)
        print('\n'.join(out))
