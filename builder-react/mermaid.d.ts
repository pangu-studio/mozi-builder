declare module 'mermaid' {
  interface MermaidAPI {
    initialize(config: Record<string, unknown>): void
    render(id: string, definition: string): Promise<{ svg: string }>
  }
  const mermaid: MermaidAPI
  export default mermaid
}
