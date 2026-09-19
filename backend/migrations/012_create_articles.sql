-- Articles & Content CMS V1: adds article_categories and articles tables
-- for managing Persian educational content.
--
-- Design notes:
--
-- * article_categories is separate from product categories - articles have
--   their own taxonomy. Slugs are unique to avoid conflicts.
--
-- * articles.content stores Markdown format (not HTML) - the frontend
--   renders safely with DOMPurify to prevent XSS. No WYSIWYG in V1.
--
-- * articles.status is either 'draft' or 'published'. Drafts are admin-only;
--   published articles are visible via the public API.
--
-- * published_at is set when an article is first published and preserved
--   on subsequent edits. Unpublishing immediately hides from public.
--
-- * category_id is nullable with ON DELETE SET NULL - deleting a category
--   leaves existing articles uncategorized rather than deleting them.
--
-- * SEO fields (seo_title, seo_description) are optional and max-length
--   validated to 70/160 characters respectively.
CREATE TABLE article_categories (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_article_categories_slug ON article_categories(slug);

CREATE TABLE articles (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    excerpt TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    cover_image_url TEXT NULL,
    category_id BIGINT NULL REFERENCES article_categories(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_at TIMESTAMPTZ NULL,
    seo_title TEXT NULL,
    seo_description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_articles_slug ON articles(slug);
CREATE INDEX idx_articles_status ON articles(status);
CREATE INDEX idx_articles_category_id ON articles(category_id);
CREATE INDEX idx_articles_published_at ON articles(published_at DESC, id DESC);