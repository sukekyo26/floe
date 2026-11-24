export const baseName = (file: string): string => file.slice(file.lastIndexOf('/') + 1);

export const dirOf = (file: string): string => (file.includes('/') ? file.slice(0, file.lastIndexOf('/')) : '');

// openKey names an open folder (slug) or a directory open in it (slug:dir): a slug has no colon
export const openKey = (slug: string, dir: string): string => (dir ? `${slug}:${dir}` : slug);
