import {defineCollection} from 'astro:content';
import {glob} from 'astro/loaders';
import {z} from 'astro/zod';
const questions=defineCollection({
  loader:glob({pattern:'*.md',base:'./.generated/interview'}),
  schema:z.object({questionId:z.string(),title:z.string(),module:z.string(),order:z.number(),difficulty:z.string(),source:z.string()}),
});
export const collections={questions};
