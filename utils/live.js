const base='http://127.0.0.1:8081';
function request(path,data){return new Promise((resolve,reject)=>wx.request({url:base+path,method:'POST',data,success:r=>r.statusCode<300?resolve(r.data):reject(Error(r.data.error||'请求失败')),fail:()=>reject(Error('无法连接8081端口，请启动Go联机服务并关闭开发工具域名校验'))}))}
function save(v){wx.setStorageSync('live-session',{code:v.room.code,token:v.token});return v}
module.exports={base,create:(name,config)=>request('/v1/rooms',{name,config}).then(save),join:(code,name)=>request('/v1/rooms/'+code+'/join',{name}).then(save)};
