import { marked } from 'marked'
import TurndownService from 'turndown'

const turndown = new TurndownService({
  headingStyle: 'atx',
  codeBlockStyle: 'fenced',
  bulletListMarker: '-',
})

marked.setOptions({
  breaks: true,
  gfm: true,
})

export const markdownToHtml = (markdown: string): string => {
  const safeMarkdown = markdown.trim().length > 0 ? markdown : ''
  const rendered = marked.parse(safeMarkdown)
  return typeof rendered === 'string' ? rendered : '<p></p>'
}

export const htmlToMarkdown = (html: string): string => {
  if (!html || html.trim() === '') {
    return ''
  }
  return turndown.turndown(html)
}
