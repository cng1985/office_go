export function sameSnapshot(left, right) {
  return JSON.stringify({ title: left.title, content: left.content }) === JSON.stringify({ title: right.title, content: right.content })
}

// applySaveSnapshot 把云端修订号接上，同时留下保存请求发出之后又改过的标题和正文。
export function applySaveSnapshot(current, submitted, server) {
  if (sameSnapshot(current, submitted)) {
    return {
      title: server.title,
      content: server.content,
      savedTitle: server.title,
      savedContent: server.content,
      pendingEdits: false,
    }
  }
  return {
    title: current.title,
    content: current.content,
    savedTitle: submitted.title,
    savedContent: submitted.content,
    pendingEdits: true,
  }
}
