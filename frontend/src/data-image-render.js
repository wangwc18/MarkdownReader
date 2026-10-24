import { documentImages, htmlImages } from './image-scaling.js';
const normalize = text => text.trim().replace(/\s+/g, ' ').toLowerCase();
// Restore SVG only in an image context, never as inline SVG/HTML.
export function restoreDataSVGImages(root, source = '') {
  const sources = documentImages(source), rendered = [...root.querySelectorAll('img')];
  const aligned = sources.length === rendered.length && rendered.every((image, i) => image.getAttribute('alt') === sources[i].label);
  for (const image of root.querySelectorAll('img:not([src])')) {
    const node = image.closest('[data-type="img"]');
    const marker = node?.querySelector('.vditor-ir__marker--link')?.textContent;
    let reference = marker;
    if (marker?.startsWith('[')) {
      const matches = sources.filter(item => item.reference && normalize(item.reference) === normalize(marker.slice(1, -1)));
      if (matches.length && matches.every(item => item.destination === matches[0].destination)) reference = matches[0].destination;
    }
    const html = image.closest('[data-type="html-block"], [data-type="html-inline"]');
    if (html) {
      const local = htmlImages(html.querySelector('code')?.textContent || '');
      const images = [...html.querySelectorAll('.vditor-ir__preview img')];
      if (local.length === images.length) reference = local[images.indexOf(image)]?.destination;
    }
    if (!reference && aligned) reference = sources[rendered.indexOf(image)].destination;
    if (!/^data:image\/svg\+xml(?:;[^,]*)?,/i.test(reference || '')) continue;
    image.setAttribute('data-media-original', reference);
    image.setAttribute('src', reference);
  }
}
