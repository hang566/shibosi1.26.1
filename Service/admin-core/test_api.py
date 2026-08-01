import requests
import json

token = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJ1c2VybmFtZSI6ImFkbWluIiwicm9sZSI6ImFkbWluIiwiaXNzIjoic2hpYm9zaS1hZG1pbi1jb3JlIiwic3ViIjoiMSIsImV4cCI6MTc4NTQxNzUyNSwibmJmIjoxNzg1NDE1NzI1LCJpYXQiOjE3ODU0MTU3MjUsImp0aSI6InlIUDgwMUNCcFM1QjhzakEwRHB0bVEifQ.JjtGL0cNL0yA9B_Um8JVeZAMh_vP5LWtGLwylVgkZ5k'
headers = {'Authorization': f'Bearer {token}'}

# 测试模块配置 API
r = requests.get('http://localhost:8084/api/v1/admin/modules', headers=headers)
print('=== Modules API ===')
print('Status:', r.status_code)
data = r.json()
if data['code'] == 0:
    modules = data['data']
    print('Module keys:', list(modules.keys()))
    for key, mod in modules.items():
        print(f"  {key}: {mod['name']} ({len(mod['pages'])} pages)")
        for page in mod['pages']:
            print(f"    - {page['id']}: {page['title']} (type: {page['type']})")

# 测试服务列表 API
r2 = requests.get('http://localhost:8084/api/v1/admin/services', headers=headers)
print('\n=== Services API ===')
print('Status:', r2.status_code)
data2 = r2.json()
if data2['code'] == 0:
    services = data2['data']
    print(f"Services count: {services['count']}")
    for svc in services['services']:
        print(f"  {svc['name']}: status={svc['status']}")
