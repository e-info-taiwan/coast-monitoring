/* Labels / keys from the existing import specification and 2024 site lookup.
   This prototype is not a new authority for unresolved site reconciliation. */
window.REEF = {
  substrates: [['HC','硬珊瑚'],['SC','軟珊瑚'],['RKC','新死珊瑚'],['NIA','藻類'],['SP','海綿'],['RC','岩石'],['RB','碎石'],['SD','沙'],['SI','泥沙'],['OT','其他']],
  methods: {
    line: {name:'底質探查',short:'底質',en:'LINE TRANSECT',description:'沿測線每 0.5 m 記一點，對照四段底質手板。',caption:'四段 × 40 點 · 含泥下底質'},
    fish: {name:'魚類探查',short:'魚類',en:'BELT · FISH',description:'依魚種與體長級距，填入四段觀察到的數量。',caption:'魚種 × 四段 · 自動計算合計'},
    invert: {name:'無脊椎動物探查',short:'無脊椎',en:'BELT · INVERTEBRATES',description:'依類群填入數量，再記錄罕見生物與環境衝擊。',caption:'類群 × 四段 · 含罕見生物與衝擊'}
  },
  fish: [
    ['Butterflyfish','蝴蝶魚',''],['Haemulidae','石鱸',''],['Snapper','笛鯛',''],['Barramundi cod','老鼠斑',''],['Humphead wrasse','蘇眉',''],['Bumphead parrotfish','龍頭鸚哥',''],['Parrotfish','鸚哥魚','>20cm'],['Moray eel','裸胸鯙',''],
    ['Grouper','石斑魚','<30cm'],['Grouper','石斑魚','30-40 cm'],['Grouper','石斑魚','40-50 cm'],['Grouper','石斑魚','50-60 cm'],['Grouper','石斑魚','>60 cm']
  ],
  invert: [
    ['Banded coral shrimp','櫻花蝦',''],['Diadema','魔鬼海膽',''],['Pencil urchin','鉛筆海膽',''],['Collector urchin','馬糞海膽',''],['Sea cucumber','海參',''],['Crown-of-thorns','棘冠海星',''],['Triton','大法螺',''],['Lobster','龍蝦',''],
    ['Giant clam','硨磲貝','<10 cm'],['Giant clam','硨磲貝','10-20 cm'],['Giant clam','硨磲貝','20-30 cm'],['Giant clam','硨磲貝','30-40 cm'],['Giant clam','硨磲貝','40-50 cm'],['Giant clam','硨磲貝','>50 cm']
  ],
  rare: [['Sharks','鯊魚',''],['Turtles','海龜',''],['Mantas','魟','']],
  impact: [
    ['Coral damage: boat/anchor','珊瑚損害：船錨','level','coral_damage'],['Coral damage: dynamite','珊瑚損害：炸魚','level','coral_damage'],['Coral damage: other','珊瑚損害：其他','level','coral_damage'],
    ['Trash: fish nets','漁業垃圾（漁網）','level','trash'],['Trash: general','一般垃圾','level','trash'],
    ['Bleaching (% of population)','珊瑚白化佔總體百分比','percent','bleaching'],['Bleaching (% of colony)','珊瑚白化佔群體百分比','percent','bleaching'],['Black Band (% colonies)','黑帶病佔珊瑚百分比','percent','disease'],['White Band (% colonies)','白帶病佔珊瑚百分比','percent','disease']
  ],
  sites: [
    ['北海岸與東北角','野柳','YeLiu'],['北海岸與東北角','潮境保育區','Chaojing protected area'],['北海岸與東北角','番仔澳',"FanCai'ao"],['北海岸與東北角','鼻頭','BiTou'],['北海岸與東北角','龍洞1.5號','LongDong1.5'],['北海岸與東北角','龍洞4號-北','LongDong4North'],['北海岸與東北角','龍洞4號-南','LongDong4South'],['北海岸與東北角','和平島','HePingIsland'],
    ['墾丁','合界','HeChei'],['墾丁','後壁湖花園','HouBiGarden'],['東海岸','石梯坪','ShiTiPing'],['東海岸','基翬','GiHaw'],['東海岸','杉原中礁','Shanyuan middle reef'],['東海岸','杉原南礁','Shanyuan southern reef'],['東海岸','杉原','ShanYuan'],['東海岸','加母子灣','KamodBay'],
    ['澎湖嶼坪','東嶼坪西側','EasternIsletW'],['澎湖嶼坪','東嶼坪東側','EasternIsletE'],['澎湖嶼坪','東嶼坪南側','EasternIsletS'],['澎湖嶼坪','西嶼坪東側','WesternIsletE'],['澎湖嶼坪','西嶼坪南側','WesternIsletS'],['澎湖嶼坪','東嶼坪北側','EasternIsletN'],['澎湖嶼坪','西嶼坪北側','WesternIsletN'],['澎湖嶼坪','杭灣','HungWan'],['澎湖嶼坪','山水港','ShanShuiHarbour'],
    ['小琉球','美人洞','BeautyCave'],['小琉球','漁埕尾','YuChenWei'],['小琉球','厚石','HouShi'],['小琉球','杉福','ShanFu'],['綠島','柴口','ChaiKou'],['綠島','公館鼻','GongGuan'],['綠島','將軍岩','JiangJunYan'],['綠島','石朗','ShiLang'],['綠島','龜灣','TurtleBay'],['綠島','大白沙','DaBaiSha'],['綠島','沈箱','ChenShiang'],
    ['蘭嶼','玉女岩','BeautyRock'],['蘭嶼','母雞岩','HenRock'],['蘭嶼','南獅','LionSouth'],['蘭嶼','東清小涼亭','Iranmeylek'],['蘭嶼','曙光礁','DawnReef'],['其他樣點','基隆嶼','KeeLungIslet'],['其他樣點','深澳','ShenAo'],['其他樣點','龍洞1號','LongDong1'],['其他樣點','新社','pateRungan'],['其他樣點','乳頭山','NippleMount'],['其他樣點','椰油灣','Yeyou'],['其他樣點','土地公廟','LandTemple'],['其他樣點','朗島港','Iraraley'],['其他樣點','虎頭坡','HuTouPo'],['其他樣點','軍艦岩','WarshipRock'],['其他樣點','雙獅岩','TwinLionsRock'],['其他樣點','坦克岩','TankRock'],['其他樣點','自來水廠','WaterStation'],['其他樣點','野銀','Ivalino'],['其他樣點','肚仔坪','DuoZaiPing'],['其他樣點','蛤板灣','GeBanBay'],['其他樣點','西嶼坪','WesternIslet'],['其他樣點','東嶼坪','EasternIslet'],['其他樣點','南鐵砧東側','NanTieZhenE'],['其他樣點','東吉港','DongJiHarbour']
  ]
};
