// Mac LAN address for simulator and phone debugging on the same Wi-Fi.
const base='http://192.168.124.19:8081';
function request(path,data){return new Promise((resolve,reject)=>wx.request({url:base+path,method:'POST',data,success:r=>r.statusCode<300?resolve(r.data):reject(Error(r.data.error||'请求失败')),fail:()=>reject(Error('无法连接192.168.124.19:8081，请确认后端已启动且手机与Mac在同一局域网'))}))}
function save(v){wx.setStorageSync('live-session',{code:v.room.code,token:v.token});return v}
module.exports={base,create:(name,config)=>request('/v1/rooms',{name,config}).then(save),join:(code,name)=>request('/v1/rooms/'+code+'/join',{name}).then(save)};
