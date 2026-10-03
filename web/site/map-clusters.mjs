// A deterministic quadtree in geographic tile space. Every parent splits only
// into its immediate next non-identical partition, independent of viewport pan.
function worldPoint(lat,lon,zoom){const scale=256*2**zoom;const s=Math.sin(Math.max(-85,Math.min(85,lat))*Math.PI/180);return [(lon+180)/360*scale,(.5-Math.log((1+s)/(1-s))/(4*Math.PI))*scale]}
export function clusterPoints(items,zoom,maxZoom=18){
 items=items.filter(item=>Number.isFinite(item.latitude)&&Number.isFinite(item.longitude));
 if(zoom>=maxZoom)return items.map(item=>({items:[item],lat:item.latitude,lon:item.longitude,count:item.event_count??1}));
 const cells=new Map();for(const item of items){if(!Number.isFinite(item.latitude)||!Number.isFinite(item.longitude))continue;const [x,y]=worldPoint(item.latitude,item.longitude,zoom),key=`${Math.floor(x/128)}:${Math.floor(y/128)}`;if(!cells.has(key))cells.set(key,[]);cells.get(key).push(item)}
 return [...cells.values()].map(group=>({items:group,lat:group.reduce((n,p)=>n+p.latitude,0)/group.length,lon:group.reduce((n,p)=>n+p.longitude,0)/group.length,count:group.reduce((n,p)=>n+(p.event_count??1),0)}));
}
export function nextClusterZoom(items,zoom,maxZoom=18){for(let z=Math.floor(zoom)+1;z<=maxZoom;z++){if(clusterPoints(items,z,maxZoom).length>1)return z}return maxZoom}
