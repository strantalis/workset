import DOMPurify from 'dompurify';
import { JSDOM } from 'jsdom';

const window = new JSDOM('').window;
const purify = DOMPurify(window);

const dirty = '<span data-action="delete" data-comment-id="123" class="diff-action-btn">Click me</span><button data-action="delete">Delete</button>';
const clean = purify.sanitize(dirty, { FORBID_ATTR: ['data-action', 'data-comment-id'] });

console.log('Clean HTML:', clean);
