// 构建期 px -> rem 转换（企业级方案）
// 设计稿基准：1920px 宽，1rem = 16px（配合 style.css 中 html { font-size: calc(100vw / 120) }）。
// - 源码继续用 px 书写（对齐设计标注），编译后自动转 rem；
// - minPixelValue: 2 —— 1px 边框/细线保持物理像素，不被缩放（发丝线不消失/变粗）；
// - exclude node_modules —— 不动 Element Plus 内部样式。
export default {
  plugins: {
    'postcss-pxtorem': {
      rootValue: 16,
      unitPrecision: 5,
      propList: ['*'],
      selectorBlackList: [],
      replace: true,
      mediaQuery: false,
      minPixelValue: 2,
      exclude: /node_modules/i,
    },
  },
}
