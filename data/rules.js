const standardPreset = {
  name: '标准瓦房店推倒胡',
  rounds: 4,
  baseScore: 1,
  allowChi: false,
  wildcards: false,
  bigHands: false,
  goldEgg: false,
  joyAndColorKong: false,
  turnSeconds: 30
}

const roomOptions = {
  rounds: [1, 4, 8],
  baseScore: [1, 2, 5, 10],
  toggles: [
    { key: 'allowChi', label: '允许吃牌', note: '仅可吃上家的数牌顺子' },
    { key: 'wildcards', label: '开启混', note: '数牌按下一张翻混，四张均有效' },
    { key: 'bigHands', label: '开启大牌', note: '可单项选择七小对、清一色等' }
  ]
}

const baseRules = [
  '136 张，万、筒、条及东南西北中发白，四人局。',
  '基础胡牌为 4 组顺子、刻子或杠，加 1 对将。字牌仅能作对子、刻子或杠。',
  '默认不吃牌。操作优先级为胡、杠、碰。一炮一单响，逆时针截胡。',
  '明杠每家 5 分，暗杠每家 10 分。本局必须有人胡才结算所有已记录杠分。',
  '胡牌坐庄，荒庄庄家连庄。点炮、自摸、杠上开花和抢杠胡均有效。'
]

module.exports = { standardPreset, roomOptions, baseRules }
