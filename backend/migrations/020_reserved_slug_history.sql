CREATE FUNCTION reserve_public_slug() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE slug_key TEXT;
BEGIN
 IF TG_OP='UPDATE' THEN
  FOR slug_key IN SELECT DISTINCT v FROM unnest(ARRAY[OLD.slug,NEW.slug]) AS v ORDER BY v LOOP
   PERFORM pg_advisory_xact_lock(hashtextextended(TG_TABLE_NAME||':'||slug_key,0));
  END LOOP;
 ELSE
  PERFORM pg_advisory_xact_lock(hashtextextended(TG_TABLE_NAME||':'||NEW.slug,0));
 END IF;
 IF TG_TABLE_NAME='products' THEN
  IF EXISTS(SELECT 1 FROM product_slug_aliases WHERE slug=NEW.slug AND product_id<>NEW.id) THEN RAISE EXCEPTION 'historical slug is reserved' USING ERRCODE='23505';END IF;
 ELSE
  IF EXISTS(SELECT 1 FROM article_slug_aliases WHERE slug=NEW.slug AND article_id<>NEW.id) THEN RAISE EXCEPTION 'historical slug is reserved' USING ERRCODE='23505';END IF;
 END IF;
 RETURN NEW;
END;$$;
CREATE TRIGGER products_reserve_slug BEFORE INSERT OR UPDATE OF slug ON products FOR EACH ROW EXECUTE FUNCTION reserve_public_slug();
CREATE TRIGGER articles_reserve_slug BEFORE INSERT OR UPDATE OF slug ON articles FOR EACH ROW EXECUTE FUNCTION reserve_public_slug();
