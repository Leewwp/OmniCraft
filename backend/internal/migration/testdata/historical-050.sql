--
-- PostgreSQL database dump
--

\restrict q5RFoRmfNOHibj2BqLpPe9IbruLOgFdzs4pFBrwkGeb7ME5fFIu6eBg2RdPRFmy

-- Dumped from database version 16.14 (Debian 16.14-1.pgdg12+1)
-- Dumped by pg_dump version 16.14 (Debian 16.14-1.pgdg12+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


--
-- Name: content_items_search_vector_update(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.content_items_search_vector_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
    NEW.search_vector :=
      setweight(to_tsvector('jiebacfg', COALESCE(NEW.title, '')), 'A') ||
      setweight(to_tsvector('jiebacfg', COALESCE(NEW.description, '')), 'B') ||
      setweight(to_tsvector('jiebacfg', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = NEW.id),
        ''
      )), 'C');
  ELSE
    NEW.search_vector :=
      setweight(to_tsvector('simple', COALESCE(NEW.title, '')), 'A') ||
      setweight(to_tsvector('simple', COALESCE(NEW.description, '')), 'B') ||
      setweight(to_tsvector('simple', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = NEW.id),
        ''
      )), 'C');
  END IF;
  RETURN NEW;
END;
$$;


--
-- Name: content_tags_search_vector_update(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.content_tags_search_vector_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  UPDATE content_items SET search_vector =
    CASE WHEN EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
      setweight(to_tsvector('jiebacfg', COALESCE(title, '')), 'A') ||
      setweight(to_tsvector('jiebacfg', COALESCE(description, '')), 'B') ||
      setweight(to_tsvector('jiebacfg', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = content_items.id), ''
      )), 'C')
    ELSE
      setweight(to_tsvector('simple', COALESCE(title, '')), 'A') ||
      setweight(to_tsvector('simple', COALESCE(description, '')), 'B') ||
      setweight(to_tsvector('simple', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM content_tags t WHERE t.content_item_id = content_items.id), ''
      )), 'C')
    END
  WHERE id = COALESCE(NEW.content_item_id, OLD.content_item_id);
  RETURN COALESCE(NEW, OLD);
END;
$$;


--
-- Name: ip_tags_search_vector_update(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.ip_tags_search_vector_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  UPDATE ips SET search_vector =
    CASE WHEN EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
      setweight(to_tsvector('jiebacfg', COALESCE(name, '')), 'A') ||
      setweight(to_tsvector('jiebacfg', COALESCE(description, '')), 'B') ||
      setweight(to_tsvector('jiebacfg', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM ip_tags t WHERE t.ip_id = ips.id), ''
      )), 'C')
    ELSE
      setweight(to_tsvector('simple', COALESCE(name, '')), 'A') ||
      setweight(to_tsvector('simple', COALESCE(description, '')), 'B') ||
      setweight(to_tsvector('simple', COALESCE(
        (SELECT string_agg(t.tag, ' ') FROM ip_tags t WHERE t.ip_id = ips.id), ''
      )), 'C')
    END
  WHERE id = COALESCE(NEW.ip_id, OLD.ip_id);
  RETURN COALESCE(NEW, OLD);
END;
$$;


--
-- Name: ips_search_vector_update(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.ips_search_vector_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba') THEN
    NEW.search_vector :=
      setweight(to_tsvector('jiebacfg', COALESCE(NEW.name, '')), 'A') ||
      setweight(to_tsvector('jiebacfg', COALESCE(NEW.description, '')), 'B') ||
      setweight(to_tsvector('jiebacfg', COALESCE(
        (SELECT string_agg(it.tag, ' ') FROM ip_tags it WHERE it.ip_id = NEW.id),
        ''
      )), 'C');
  ELSE
    NEW.search_vector :=
      setweight(to_tsvector('simple', COALESCE(NEW.name, '')), 'A') ||
      setweight(to_tsvector('simple', COALESCE(NEW.description, '')), 'B') ||
      setweight(to_tsvector('simple', COALESCE(
        (SELECT string_agg(it.tag, ' ') FROM ip_tags it WHERE it.ip_id = NEW.id),
        ''
      )), 'C');
  END IF;
  RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: agent_conversations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_conversations (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    context_type character varying(50) DEFAULT ''::character varying NOT NULL,
    context_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: agent_conversations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.agent_conversations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: agent_conversations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.agent_conversations_id_seq OWNED BY public.agent_conversations.id;


--
-- Name: agent_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_messages (
    id bigint NOT NULL,
    conversation_id bigint NOT NULL,
    role character varying(20) NOT NULL,
    content text,
    tool_calls jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT agent_messages_role_check CHECK (((role)::text = ANY ((ARRAY['system'::character varying, 'user'::character varying, 'assistant'::character varying, 'tool'::character varying])::text[])))
);


--
-- Name: agent_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.agent_messages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: agent_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.agent_messages_id_seq OWNED BY public.agent_messages.id;


--
-- Name: ai_review_records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_review_records (
    id bigint NOT NULL,
    target_type character varying(20) NOT NULL,
    target_id bigint NOT NULL,
    provider character varying(50) DEFAULT 'aliyun'::character varying NOT NULL,
    result character varying(20) NOT NULL,
    raw_response jsonb,
    scanned_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ai_review_records_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_review_records_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_review_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_review_records_id_seq OWNED BY public.ai_review_records.id;


--
-- Name: appeals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.appeals (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    target_type character varying(20) NOT NULL,
    target_id bigint NOT NULL,
    reason text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    admin_response text,
    resolved_by bigint,
    resolved_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT appeals_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT appeals_target_type_check CHECK (((target_type)::text = ANY ((ARRAY['content'::character varying, 'comment'::character varying])::text[])))
);


--
-- Name: appeals_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.appeals_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: appeals_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.appeals_id_seq OWNED BY public.appeals.id;


--
-- Name: author_blocklist; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.author_blocklist (
    author_id bigint NOT NULL,
    blocked_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: browse_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browse_history (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    content_item_id bigint NOT NULL,
    viewed_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: browse_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browse_history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browse_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browse_history_id_seq OWNED BY public.browse_history.id;


--
-- Name: categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.categories (
    id bigint NOT NULL,
    zone character varying(20) NOT NULL,
    level character varying(20) NOT NULL,
    parent_id bigint,
    name_i18n jsonb DEFAULT '{}'::jsonb NOT NULL,
    slug character varying(100) NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: categories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.categories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: categories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.categories_id_seq OWNED BY public.categories.id;


--
-- Name: comments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comments (
    id bigint NOT NULL,
    content_item_id bigint,
    discussion_id bigint,
    parent_id bigint,
    author_id bigint NOT NULL,
    body text NOT NULL,
    status character varying(20) DEFAULT 'published'::character varying NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    target_type character varying(20),
    target_id bigint,
    content text,
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: comments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.comments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: comments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.comments_id_seq OWNED BY public.comments.id;


--
-- Name: content_attachments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_attachments (
    id bigint NOT NULL,
    content_item_id bigint NOT NULL,
    file_type character varying(30) NOT NULL,
    oss_key text NOT NULL,
    file_size bigint,
    mime_type character varying(100),
    duration_sec integer,
    width integer,
    height integer,
    is_primary boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: content_attachments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.content_attachments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: content_attachments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.content_attachments_id_seq OWNED BY public.content_attachments.id;


--
-- Name: content_contributors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_contributors (
    content_item_id bigint NOT NULL,
    user_id bigint NOT NULL,
    pr_count integer DEFAULT 1 NOT NULL,
    first_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: content_embeddings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_embeddings (
    content_item_id bigint NOT NULL,
    embedding public.vector(1536) NOT NULL,
    embedded_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: content_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_items (
    id bigint NOT NULL,
    title character varying(500) NOT NULL,
    author_id bigint NOT NULL,
    zone character varying(10) NOT NULL,
    ip_id bigint,
    category character varying(50),
    content_type character varying(20) NOT NULL,
    cover_image_url text,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    view_count bigint DEFAULT 0 NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    dislike_count integer DEFAULT 0 NOT NULL,
    is_public boolean DEFAULT true NOT NULL,
    allow_copy boolean DEFAULT true NOT NULL,
    agent_enabled boolean DEFAULT false NOT NULL,
    is_paid boolean DEFAULT false NOT NULL,
    price numeric(10,2) DEFAULT 0,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    description text,
    source_original_id bigint,
    ban_reason text,
    download_count integer DEFAULT 0 NOT NULL,
    search_vector tsvector,
    hot_score double precision DEFAULT 0,
    rating_score double precision DEFAULT 0
);


--
-- Name: content_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.content_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: content_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.content_items_id_seq OWNED BY public.content_items.id;


--
-- Name: content_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_tags (
    content_item_id bigint NOT NULL,
    tag character varying(50) NOT NULL
);


--
-- Name: content_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_versions (
    id bigint NOT NULL,
    content_item_id bigint NOT NULL,
    parent_version_id bigint,
    author_id bigint NOT NULL,
    version_number integer NOT NULL,
    storage_type character varying(10) NOT NULL,
    storage_key text,
    diff_summary text,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    is_latest boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: content_versions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.content_versions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: content_versions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.content_versions_id_seq OWNED BY public.content_versions.id;


--
-- Name: conversation_participants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversation_participants (
    conversation_id bigint NOT NULL,
    user_id bigint NOT NULL,
    last_read_at timestamp with time zone,
    unread_count integer DEFAULT 0 NOT NULL,
    left_at timestamp with time zone
);


--
-- Name: conversations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversations (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: conversations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.conversations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: conversations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.conversations_id_seq OWNED BY public.conversations.id;


--
-- Name: discussions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discussions (
    id bigint NOT NULL,
    ip_id bigint,
    content_item_id bigint,
    author_id bigint NOT NULL,
    title character varying(500) NOT NULL,
    body text,
    status character varying(20) DEFAULT 'published'::character varying NOT NULL,
    view_count bigint DEFAULT 0 NOT NULL,
    reply_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    is_pinned boolean DEFAULT false NOT NULL,
    last_active_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: discussions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discussions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discussions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discussions_id_seq OWNED BY public.discussions.id;


--
-- Name: favorites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.favorites (
    user_id bigint NOT NULL,
    content_item_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: follows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.follows (
    id bigint NOT NULL,
    follower_id bigint NOT NULL,
    target_type character varying(20) NOT NULL,
    target_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT follows_target_type_check CHECK (((target_type)::text = ANY ((ARRAY['user'::character varying, 'ip'::character varying])::text[])))
);


--
-- Name: follows_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.follows_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: follows_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.follows_id_seq OWNED BY public.follows.id;


--
-- Name: ip_review_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ip_review_logs (
    id bigint NOT NULL,
    ip_id bigint NOT NULL,
    reviewer_id bigint,
    action character varying(20) NOT NULL,
    reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ip_review_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ip_review_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ip_review_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ip_review_logs_id_seq OWNED BY public.ip_review_logs.id;


--
-- Name: ip_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ip_tags (
    ip_id bigint NOT NULL,
    tag character varying(50) NOT NULL
);


--
-- Name: ips; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ips (
    id bigint NOT NULL,
    name character varying(255) NOT NULL,
    slug character varying(255) NOT NULL,
    description text,
    cover_url text,
    category character varying(50),
    creator_id bigint,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    search_vector tsvector,
    popularity_score double precision DEFAULT 0
);


--
-- Name: ips_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ips_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ips_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ips_id_seq OWNED BY public.ips.id;


--
-- Name: judge_cases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.judge_cases (
    id bigint NOT NULL,
    target_type character varying(20) NOT NULL,
    target_id bigint NOT NULL,
    status character varying(20) DEFAULT 'open'::character varying NOT NULL,
    vote_approve integer DEFAULT 0 NOT NULL,
    vote_reject integer DEFAULT 0 NOT NULL,
    min_votes integer DEFAULT 20 NOT NULL,
    closed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: judge_cases_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.judge_cases_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: judge_cases_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.judge_cases_id_seq OWNED BY public.judge_cases.id;


--
-- Name: judge_exam_records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.judge_exam_records (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    content_type character varying(50) NOT NULL,
    score integer NOT NULL,
    total integer NOT NULL,
    passed boolean NOT NULL,
    taken_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: judge_exam_records_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.judge_exam_records_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: judge_exam_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.judge_exam_records_id_seq OWNED BY public.judge_exam_records.id;


--
-- Name: judge_qualifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.judge_qualifications (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    content_type character varying(50) NOT NULL,
    qualified_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    is_active boolean DEFAULT true NOT NULL
);


--
-- Name: judge_qualifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.judge_qualifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: judge_qualifications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.judge_qualifications_id_seq OWNED BY public.judge_qualifications.id;


--
-- Name: judge_questions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.judge_questions (
    id bigint NOT NULL,
    content_type character varying(50) NOT NULL,
    source_case_id bigint,
    question_data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_by character varying(20) DEFAULT 'admin'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: judge_questions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.judge_questions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: judge_questions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.judge_questions_id_seq OWNED BY public.judge_questions.id;


--
-- Name: judge_reason_votes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.judge_reason_votes (
    id bigint NOT NULL,
    reason_owner_vote_id bigint NOT NULL,
    voter_id bigint NOT NULL,
    vote_type character varying(10) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT judge_reason_votes_vote_type_check CHECK (((vote_type)::text = ANY ((ARRAY['up'::character varying, 'down'::character varying])::text[])))
);


--
-- Name: judge_reason_votes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.judge_reason_votes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: judge_reason_votes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.judge_reason_votes_id_seq OWNED BY public.judge_reason_votes.id;


--
-- Name: judge_votes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.judge_votes (
    id bigint NOT NULL,
    case_id bigint NOT NULL,
    judge_id bigint NOT NULL,
    vote character varying(10) NOT NULL,
    reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: judge_votes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.judge_votes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: judge_votes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.judge_votes_id_seq OWNED BY public.judge_votes.id;


--
-- Name: llm_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.llm_configs (
    id bigint NOT NULL,
    config_name character varying(100) NOT NULL,
    provider_type character varying(50) NOT NULL,
    api_base character varying(500),
    model character varying(100) NOT NULL,
    api_key_enc text,
    is_active boolean DEFAULT false NOT NULL,
    extra_params jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: llm_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.llm_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: llm_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.llm_configs_id_seq OWNED BY public.llm_configs.id;


--
-- Name: messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.messages (
    id bigint NOT NULL,
    conversation_id bigint NOT NULL,
    sender_id bigint NOT NULL,
    body text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.messages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.messages_id_seq OWNED BY public.messages.id;


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    type character varying(50) NOT NULL,
    channel character varying(20) NOT NULL,
    title character varying(500),
    body text,
    target_type character varying(50),
    target_id bigint,
    sender_id bigint,
    is_read boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT notifications_channel_check CHECK (((channel)::text = ANY ((ARRAY['reply'::character varying, 'like'::character varying, 'system'::character varying, 'pr'::character varying, 'follow'::character varying])::text[])))
);


--
-- Name: notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: notifications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.notifications_id_seq OWNED BY public.notifications.id;


--
-- Name: oauth_accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.oauth_accounts (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider character varying(20) NOT NULL,
    provider_uid character varying(255) NOT NULL,
    access_token text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: oauth_accounts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.oauth_accounts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: oauth_accounts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.oauth_accounts_id_seq OWNED BY public.oauth_accounts.id;


--
-- Name: password_reset_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.password_reset_tokens (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    token character varying(255) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: password_reset_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.password_reset_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: password_reset_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.password_reset_tokens_id_seq OWNED BY public.password_reset_tokens.id;


--
-- Name: pull_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pull_requests (
    id bigint NOT NULL,
    content_item_id bigint NOT NULL,
    submitter_id bigint NOT NULL,
    base_version_id bigint NOT NULL,
    proposed_version_id bigint,
    status character varying(20) DEFAULT 'open'::character varying NOT NULL,
    message text,
    reject_reason text,
    resolved_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: pull_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.pull_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: pull_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.pull_requests_id_seq OWNED BY public.pull_requests.id;


--
-- Name: reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reactions (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    target_type character varying(20) NOT NULL,
    target_id bigint NOT NULL,
    reaction character varying(10) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: reactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reactions_id_seq OWNED BY public.reactions.id;


--
-- Name: rehab_completions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rehab_completions (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    course_id bigint NOT NULL,
    completed_at timestamp with time zone DEFAULT now(),
    started_at timestamp with time zone
);


--
-- Name: rehab_completions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rehab_completions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rehab_completions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rehab_completions_id_seq OWNED BY public.rehab_completions.id;


--
-- Name: rehab_courses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rehab_courses (
    id bigint NOT NULL,
    violation_type character varying(100) NOT NULL,
    content_i18n jsonb DEFAULT '{}'::jsonb NOT NULL,
    min_reading_sec integer DEFAULT 60 NOT NULL,
    reward_points integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: rehab_courses_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rehab_courses_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rehab_courses_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rehab_courses_id_seq OWNED BY public.rehab_courses.id;


--
-- Name: reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reports (
    id bigint NOT NULL,
    reporter_id bigint NOT NULL,
    target_type character varying(20) NOT NULL,
    target_id bigint NOT NULL,
    reason character varying(100) NOT NULL,
    detail text,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: reports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reports_id_seq OWNED BY public.reports.id;


--
-- Name: reputation_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reputation_logs (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    delta integer NOT NULL,
    reason character varying(100) NOT NULL,
    related_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: reputation_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reputation_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reputation_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reputation_logs_id_seq OWNED BY public.reputation_logs.id;


--
-- Name: saved_searches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.saved_searches (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    name character varying(200) NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: saved_searches_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.saved_searches_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: saved_searches_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.saved_searches_id_seq OWNED BY public.saved_searches.id;


--
-- Name: schema_migration_attempts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migration_attempts (
    id bigint NOT NULL,
    version integer NOT NULL,
    filename text NOT NULL,
    checksum text NOT NULL,
    status text NOT NULL,
    error_class text DEFAULT ''::text NOT NULL,
    error_digest text DEFAULT ''::text NOT NULL,
    note text DEFAULT ''::text NOT NULL,
    attempted_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT schema_migration_attempts_status_check CHECK ((status = ANY (ARRAY['started'::text, 'succeeded'::text, 'failed'::text, 'reconciled'::text])))
);


--
-- Name: schema_migration_attempts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.schema_migration_attempts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: schema_migration_attempts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.schema_migration_attempts_id_seq OWNED BY public.schema_migration_attempts.id;


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version integer NOT NULL,
    filename text NOT NULL,
    checksum text NOT NULL,
    applied_at timestamp with time zone NOT NULL
);


--
-- Name: tag_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_groups (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    name character varying(100) NOT NULL,
    tags text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tag_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_groups_id_seq OWNED BY public.tag_groups.id;


--
-- Name: tag_suggestions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_suggestions (
    id bigint NOT NULL,
    content_item_id bigint NOT NULL,
    user_id bigint NOT NULL,
    tag character varying(100) NOT NULL,
    action character varying(10) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tag_suggestions_action_check CHECK (((action)::text = ANY ((ARRAY['add'::character varying, 'remove'::character varying])::text[]))),
    CONSTRAINT tag_suggestions_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'rejected'::character varying, 'reported'::character varying])::text[])))
);


--
-- Name: tag_suggestions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_suggestions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_suggestions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_suggestions_id_seq OWNED BY public.tag_suggestions.id;


--
-- Name: tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tags (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    category character varying(50) DEFAULT ''::character varying NOT NULL,
    usage_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tags_id_seq OWNED BY public.tags.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    username character varying(64) NOT NULL,
    avatar_url text,
    bio text,
    reputation integer DEFAULT 10 NOT NULL,
    preferred_locale character varying(10) DEFAULT 'zh-CN'::character varying NOT NULL,
    role character varying(20) DEFAULT 'user'::character varying NOT NULL,
    is_banned boolean DEFAULT false NOT NULL,
    ban_reason text,
    support_info jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    email_verified_at timestamp with time zone,
    accepted_terms_version character varying(32),
    accepted_terms_at timestamp with time zone,
    accepted_privacy_version character varying(32),
    accepted_privacy_at timestamp with time zone
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: agent_conversations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_conversations ALTER COLUMN id SET DEFAULT nextval('public.agent_conversations_id_seq'::regclass);


--
-- Name: agent_messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_messages ALTER COLUMN id SET DEFAULT nextval('public.agent_messages_id_seq'::regclass);


--
-- Name: ai_review_records id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_review_records ALTER COLUMN id SET DEFAULT nextval('public.ai_review_records_id_seq'::regclass);


--
-- Name: appeals id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.appeals ALTER COLUMN id SET DEFAULT nextval('public.appeals_id_seq'::regclass);


--
-- Name: browse_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browse_history ALTER COLUMN id SET DEFAULT nextval('public.browse_history_id_seq'::regclass);


--
-- Name: categories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories ALTER COLUMN id SET DEFAULT nextval('public.categories_id_seq'::regclass);


--
-- Name: comments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments ALTER COLUMN id SET DEFAULT nextval('public.comments_id_seq'::regclass);


--
-- Name: content_attachments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_attachments ALTER COLUMN id SET DEFAULT nextval('public.content_attachments_id_seq'::regclass);


--
-- Name: content_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_items ALTER COLUMN id SET DEFAULT nextval('public.content_items_id_seq'::regclass);


--
-- Name: content_versions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_versions ALTER COLUMN id SET DEFAULT nextval('public.content_versions_id_seq'::regclass);


--
-- Name: conversations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversations ALTER COLUMN id SET DEFAULT nextval('public.conversations_id_seq'::regclass);


--
-- Name: discussions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discussions ALTER COLUMN id SET DEFAULT nextval('public.discussions_id_seq'::regclass);


--
-- Name: follows id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows ALTER COLUMN id SET DEFAULT nextval('public.follows_id_seq'::regclass);


--
-- Name: ip_review_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ip_review_logs ALTER COLUMN id SET DEFAULT nextval('public.ip_review_logs_id_seq'::regclass);


--
-- Name: ips id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ips ALTER COLUMN id SET DEFAULT nextval('public.ips_id_seq'::regclass);


--
-- Name: judge_cases id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_cases ALTER COLUMN id SET DEFAULT nextval('public.judge_cases_id_seq'::regclass);


--
-- Name: judge_exam_records id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_exam_records ALTER COLUMN id SET DEFAULT nextval('public.judge_exam_records_id_seq'::regclass);


--
-- Name: judge_qualifications id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_qualifications ALTER COLUMN id SET DEFAULT nextval('public.judge_qualifications_id_seq'::regclass);


--
-- Name: judge_questions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_questions ALTER COLUMN id SET DEFAULT nextval('public.judge_questions_id_seq'::regclass);


--
-- Name: judge_reason_votes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_reason_votes ALTER COLUMN id SET DEFAULT nextval('public.judge_reason_votes_id_seq'::regclass);


--
-- Name: judge_votes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_votes ALTER COLUMN id SET DEFAULT nextval('public.judge_votes_id_seq'::regclass);


--
-- Name: llm_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_configs ALTER COLUMN id SET DEFAULT nextval('public.llm_configs_id_seq'::regclass);


--
-- Name: messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages ALTER COLUMN id SET DEFAULT nextval('public.messages_id_seq'::regclass);


--
-- Name: notifications id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications ALTER COLUMN id SET DEFAULT nextval('public.notifications_id_seq'::regclass);


--
-- Name: oauth_accounts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_accounts ALTER COLUMN id SET DEFAULT nextval('public.oauth_accounts_id_seq'::regclass);


--
-- Name: password_reset_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens ALTER COLUMN id SET DEFAULT nextval('public.password_reset_tokens_id_seq'::regclass);


--
-- Name: pull_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests ALTER COLUMN id SET DEFAULT nextval('public.pull_requests_id_seq'::regclass);


--
-- Name: reactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reactions ALTER COLUMN id SET DEFAULT nextval('public.reactions_id_seq'::regclass);


--
-- Name: rehab_completions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rehab_completions ALTER COLUMN id SET DEFAULT nextval('public.rehab_completions_id_seq'::regclass);


--
-- Name: rehab_courses id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rehab_courses ALTER COLUMN id SET DEFAULT nextval('public.rehab_courses_id_seq'::regclass);


--
-- Name: reports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reports ALTER COLUMN id SET DEFAULT nextval('public.reports_id_seq'::regclass);


--
-- Name: reputation_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reputation_logs ALTER COLUMN id SET DEFAULT nextval('public.reputation_logs_id_seq'::regclass);


--
-- Name: saved_searches id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_searches ALTER COLUMN id SET DEFAULT nextval('public.saved_searches_id_seq'::regclass);


--
-- Name: schema_migration_attempts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migration_attempts ALTER COLUMN id SET DEFAULT nextval('public.schema_migration_attempts_id_seq'::regclass);


--
-- Name: tag_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_groups ALTER COLUMN id SET DEFAULT nextval('public.tag_groups_id_seq'::regclass);


--
-- Name: tag_suggestions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_suggestions ALTER COLUMN id SET DEFAULT nextval('public.tag_suggestions_id_seq'::regclass);


--
-- Name: tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags ALTER COLUMN id SET DEFAULT nextval('public.tags_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Data for Name: agent_conversations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: agent_messages; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ai_review_records; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: appeals; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: author_blocklist; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: browse_history; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: categories; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.categories VALUES (1, 'fanwork', 'ip_category', NULL, '{"en": "Gaming", "zh": "游戏"}', 'gaming', 1, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (2, 'fanwork', 'ip_category', NULL, '{"en": "Anime", "zh": "动漫"}', 'anime', 2, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (3, 'fanwork', 'ip_category', NULL, '{"en": "Music", "zh": "音乐"}', 'music', 3, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (4, 'fanwork', 'ip_category', NULL, '{"en": "Film/TV", "zh": "影视"}', 'film_tv', 4, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (5, 'fanwork', 'ip_category', NULL, '{"en": "Literature", "zh": "文学"}', 'literature', 5, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (6, 'fanwork', 'ip_category', NULL, '{"en": "Other", "zh": "其他"}', 'other', 99, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (7, 'original', 'primary', NULL, '{"en": "Film/TV", "zh": "影视"}', 'film_tv_orig', 1, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (8, 'original', 'primary', NULL, '{"en": "Gaming", "zh": "游戏"}', 'gaming_orig', 2, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (9, 'original', 'primary', NULL, '{"en": "Literature", "zh": "文学"}', 'literature_orig', 3, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (10, 'original', 'primary', NULL, '{"en": "Pet", "zh": "宠物"}', 'pet', 4, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (11, 'original', 'primary', NULL, '{"en": "Food", "zh": "美食"}', 'food', 5, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (12, 'original', 'primary', NULL, '{"en": "Beauty/Fashion", "zh": "美妆穿搭"}', 'beauty_fashion', 6, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (13, 'original', 'primary', NULL, '{"en": "Home", "zh": "家居"}', 'home', 7, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (14, 'original', 'primary', NULL, '{"en": "Tech/Digital", "zh": "数码科技"}', 'tech_digital', 8, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (15, 'original', 'primary', NULL, '{"en": "Travel", "zh": "旅行"}', 'travel', 9, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (16, 'original', 'primary', NULL, '{"en": "Sports", "zh": "运动"}', 'sports', 10, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.categories VALUES (17, 'original', 'primary', NULL, '{"en": "Productivity", "zh": "效率"}', 'productivity', 11, true, '2026-08-04 17:55:00.293956+00', '2026-08-04 17:55:00.293956+00');


--
-- Data for Name: comments; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: content_attachments; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: content_contributors; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: content_embeddings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: content_items; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: content_tags; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: content_versions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: conversation_participants; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: conversations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: discussions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: favorites; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: follows; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ip_review_logs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ip_tags; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ips; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: judge_cases; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: judge_exam_records; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: judge_qualifications; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: judge_questions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: judge_reason_votes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: judge_votes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: llm_configs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: messages; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notifications; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: oauth_accounts; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: password_reset_tokens; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pull_requests; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: reactions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rehab_completions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rehab_courses; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.rehab_courses VALUES (1, 'malicious_report_tag', '{"en": "# Malicious Tag Report\n\nPlease do not abuse the report feature...", "zh": "# 恶意标签举报行为说明\n\n请勿滥用举报功能进行恶意操作..."}', 120, 1, '2026-08-04 17:55:00.291526+00');
INSERT INTO public.rehab_courses VALUES (2, 'malicious_comment', '{"en": "# Comment Guidelines\n\nPlease be respectful...", "zh": "# 评论规范\n\n请文明发言，尊重他人..."}', 90, 1, '2026-08-04 17:55:00.291526+00');
INSERT INTO public.rehab_courses VALUES (3, 'malicious_contribution', '{"en": "# Contribution Guidelines\n\nDo not submit invalid or destructive PRs...", "zh": "# 协作贡献规范\n\n请勿提交无效或破坏性 PR..."}', 90, 1, '2026-08-04 17:55:00.291526+00');
INSERT INTO public.rehab_courses VALUES (4, 'malicious_report_comment', '{"en": "# Report Guidelines\n\nUse the report function responsibly...", "zh": "# 评论举报规范\n\n请合理使用举报功能..."}', 60, 1, '2026-08-04 17:55:00.291526+00');
INSERT INTO public.rehab_courses VALUES (5, 'judge_error', '{"en": "# Judge Responsibilities\n\nHigh error rates affect community fairness...", "zh": "# 判官职责说明\n\n投票错误率过高将影响社区公正性..."}', 180, 2, '2026-08-04 17:55:00.291526+00');


--
-- Data for Name: reports; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: reputation_logs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: saved_searches; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: schema_migration_attempts; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.schema_migration_attempts VALUES (1, 47, '047_pg_trgm_indexes.sql', '1370f7d9a607850b7a5167dac250cbdcc7ff33fe395b365ee1f203a1dbd745a8', 'succeeded', '', '', '', '2026-08-04 17:55:00.337202+00');
INSERT INTO public.schema_migration_attempts VALUES (2, 49, '049_search_trigram_fallback.sql', '155e0765feaf8f1803b697bfe1e9bb4c50f918aed4879b040e834233a3040190', 'succeeded', '', '', '', '2026-08-04 17:55:00.34598+00');


--
-- Data for Name: schema_migrations; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.schema_migrations VALUES (1, '001_users.sql', 'eed6f8a9a3cc72209547dd2dce8bea253423c4f7c4bcc5ef8b823d02dd25ec1e', '2026-08-04 17:55:00.152897+00');
INSERT INTO public.schema_migrations VALUES (2, '002_judge_qualifications.sql', '010a656abd83621777cbdd4237e578d29be5cfd152ffcf40132fc85acfb45799', '2026-08-04 17:55:00.161773+00');
INSERT INTO public.schema_migrations VALUES (3, '003_reputation_logs.sql', '61615480ad62602bc1ca88e27c49154c17ab371a1c8d94ba0c4f172a4fa12b37', '2026-08-04 17:55:00.167554+00');
INSERT INTO public.schema_migrations VALUES (4, '004_oauth_accounts.sql', 'b173b458284d160eaa9f10651e5eba8d68ee14c33964240674a1516007a34d00', '2026-08-04 17:55:00.171476+00');
INSERT INTO public.schema_migrations VALUES (5, '005_ips.sql', '4b4939e92652e9150c697ecfa8d519795f1099dd78bd5276c22ed53ad6cab18d', '2026-08-04 17:55:00.176623+00');
INSERT INTO public.schema_migrations VALUES (6, '006_content_items.sql', 'ae4108feba29b4359681d650a85eec2ef32b9ca7c11d3c9bbb078100a6662758', '2026-08-04 17:55:00.18724+00');
INSERT INTO public.schema_migrations VALUES (7, '007_content_attachments.sql', 'f8b76f46bd8412e5964fbd613f158366876a3d822d0ef9457cba2dd9244f37a8', '2026-08-04 17:55:00.195234+00');
INSERT INTO public.schema_migrations VALUES (8, '008_content_tags.sql', '71bce8ffe6f7567ff46b39889c7969041eeb79e01649916e4409173a76d39e52', '2026-08-04 17:55:00.198487+00');
INSERT INTO public.schema_migrations VALUES (9, '009_content_versions.sql', '097a9055646e5bd1622a96d29bb3acb918f07d0785c73b125ff108d47836cfca', '2026-08-04 17:55:00.20032+00');
INSERT INTO public.schema_migrations VALUES (10, '010_pull_requests.sql', '0373520ce57ac6f9336ca7e62d9a00bc8bfaa3dd505df51143dfaad977b18c42', '2026-08-04 17:55:00.205695+00');
INSERT INTO public.schema_migrations VALUES (11, '011_social.sql', 'd00892cd0e32eb03a985d149f40281f452fd7b328cfaf637f55b090e8a2b98bf', '2026-08-04 17:55:00.211923+00');
INSERT INTO public.schema_migrations VALUES (12, '012_reports.sql', '704af109937c090ce720a36712fffd45facbdd8424af2cc678fd8f9e2c347007', '2026-08-04 17:55:00.230439+00');
INSERT INTO public.schema_migrations VALUES (13, '013_ai_review.sql', 'e19e26d1abc7538c5d76ffdb5eaccda3d9707db3e21c68fd3c73ad54920ecf5c', '2026-08-04 17:55:00.234862+00');
INSERT INTO public.schema_migrations VALUES (14, '014_judge.sql', '6872dab7de2956afb1068ecddb26879820e67115376bfc038cdcdc3c51642ac7', '2026-08-04 17:55:00.237846+00');
INSERT INTO public.schema_migrations VALUES (15, '015_tags.sql', 'ab5646d3fe3046b48302d99267a24f97dc0aef6f8e1cbcc9c66970ce9a2c7ff3', '2026-08-04 17:55:00.248701+00');
INSERT INTO public.schema_migrations VALUES (16, '016_tag_suggestions.sql', '36cb570c1dd7678c7e04939175e4db928993ab1f77761ca75668a968be4c86cd', '2026-08-04 17:55:00.251742+00');
INSERT INTO public.schema_migrations VALUES (17, '017_tag_groups.sql', '5533b3cfe57369abcb640ac167a878ef74021fd5bc901bf822d8fee8a5d9f945', '2026-08-04 17:55:00.255767+00');
INSERT INTO public.schema_migrations VALUES (18, '018_saved_searches.sql', 'c4a519566a58bdc083869ea6308ff947eaa975b5f841aba935123ceeb3794661', '2026-08-04 17:55:00.258388+00');
INSERT INTO public.schema_migrations VALUES (19, '019_pgvector.sql', '9e9b2cfec47519f49ee73cb533c459e22f8ca54fe5ba1cbec59f3d5883fe191c', '2026-08-04 17:55:00.261131+00');
INSERT INTO public.schema_migrations VALUES (20, '020_agent_conversations.sql', '8f92a564bbd4cd315182dcd5781537f98f86a71d42df83d97a5289f43a660818', '2026-08-04 17:55:00.267661+00');
INSERT INTO public.schema_migrations VALUES (21, '021_agent_messages.sql', '1e872a500bcec04df8ab255734560048ae630d749fcb13c160dd816feb80e1e4', '2026-08-04 17:55:00.270463+00');
INSERT INTO public.schema_migrations VALUES (22, '022_content_embeddings.sql', 'fd2064879e920be74d36ea265e29d635159612f81170ebaf59141769e66f9125', '2026-08-04 17:55:00.273467+00');
INSERT INTO public.schema_migrations VALUES (23, '023_notifications.sql', 'dd768794756232f447966fefc69fcbffc55b953f90dee69f964acc77b5104f47', '2026-08-04 17:55:00.278193+00');
INSERT INTO public.schema_migrations VALUES (24, '024_conversations.sql', '6b65506489784f31bb7627f106ccf5e6a49eca7c858a73b886be8da66a4273e4', '2026-08-04 17:55:00.281852+00');
INSERT INTO public.schema_migrations VALUES (25, '025_rehab.sql', '57952fd8a17056cad2325c9ec95d2c9aa97e321b5cc57b4fd3b3044f1a1e8e3d', '2026-08-04 17:55:00.287165+00');
INSERT INTO public.schema_migrations VALUES (26, '026_rehab_seed.sql', '2a66a9a55c59b0063e216e02ee1eabc3c5c616c9a6cac4fbf3d38c431e58f227', '2026-08-04 17:55:00.291526+00');
INSERT INTO public.schema_migrations VALUES (27, '027_content_category.sql', 'bcdd9235c1c2756ad3ed77e1e0e5a37c1b10aa46cb81200d9b25b52433dd079a', '2026-08-04 17:55:00.292749+00');
INSERT INTO public.schema_migrations VALUES (28, '028_categories.sql', '44f19e972c1ae985ea1091cc9769e5bea11690e3b84adfc4b40b72e406542455', '2026-08-04 17:55:00.293956+00');
INSERT INTO public.schema_migrations VALUES (29, '029_judge_reason.sql', '3bbdda60b8e88dcfc3d83950535f869c1bc1371227fbc3e3100a2e77ada3e081', '2026-08-04 17:55:00.298306+00');
INSERT INTO public.schema_migrations VALUES (30, '030_add_support_info_to_users.sql', 'f5d445fc936d2f57b66945b9c96ef9b825beeadce448461ec287c53784a7c2ff', '2026-08-04 17:55:00.300242+00');
INSERT INTO public.schema_migrations VALUES (31, '031_create_llm_configs.sql', 'f777a3b65381be29b5b6961c7b370c282600bfc5d831833b2aa3a2f2253e34b4', '2026-08-04 17:55:00.301309+00');
INSERT INTO public.schema_migrations VALUES (32, '032_browse_history.sql', 'a31968076b70712ff58f355ec78348c71e2d1de79db598399336e41818a75b0e', '2026-08-04 17:55:00.303992+00');
INSERT INTO public.schema_migrations VALUES (33, '033_follows.sql', '4348bd9ef0ad70244b7b6c121e3f5ddf0050e38812f4388ce79a6acbd40c72f6', '2026-08-04 17:55:00.305607+00');
INSERT INTO public.schema_migrations VALUES (34, '034_appeals.sql', 'cfebf03964a1db1e82ac5f1e2ba1cf44addb3f47b83a354656f838cff54a4f2d', '2026-08-04 17:55:00.307319+00');
INSERT INTO public.schema_migrations VALUES (35, '035_discussions.sql', 'b0e0f4113c4336637099eb251c3416a30f330173b5b548f8b2908678a7a6902d', '2026-08-04 17:55:00.309412+00');
INSERT INTO public.schema_migrations VALUES (36, '036_content_source_original.sql', '596a392920ced2ec8fd69b334522467f87c998ee47933a96062bf28b03a4e0ca', '2026-08-04 17:55:00.311947+00');
INSERT INTO public.schema_migrations VALUES (37, '037_rehab_started_at.sql', 'b0f6f1d75879839974cb2350117a7a28d8aae42312f05acff5636a69492032c7', '2026-08-04 17:55:00.314002+00');
INSERT INTO public.schema_migrations VALUES (38, '038_notification_channels.sql', '125c161783141135d6b911da7d1ecfaf93f280500db18141f385478d1b8cbb77', '2026-08-04 17:55:00.315431+00');
INSERT INTO public.schema_migrations VALUES (39, '039_conversation_unread_count.sql', '4520f722f18e29188223956f39576c0d6b12d2b3a8ba58e059dff7afb1b8fe5e', '2026-08-04 17:55:00.317088+00');
INSERT INTO public.schema_migrations VALUES (40, '040_p0_fixes.sql', '1616d2760b4308aa9e7e8a60e7e0836a8506b35286f555063ea88c9833ec3e68', '2026-08-04 17:55:00.318072+00');
INSERT INTO public.schema_migrations VALUES (41, '041_content_search_vector.sql', '9fcc62739e20e1a345b8bce5e628c3b66113cbddf919d7b9d72536e23adc679f', '2026-08-04 17:55:00.323115+00');
INSERT INTO public.schema_migrations VALUES (42, '042_ips_search_vector.sql', '23e58b0d9e3f1f638d760872c6153492a56161f9d61a5b9489a7131fe6c162fc', '2026-08-04 17:55:00.325723+00');
INSERT INTO public.schema_migrations VALUES (43, '043_comment_columns.sql', 'b4bc99be2eb162f499f7c369b92d63f2e7b81a44526897295d9e7fd2615832f7', '2026-08-04 17:55:00.327768+00');
INSERT INTO public.schema_migrations VALUES (44, '044_indexes_and_fixes.sql', 'b59d0be9c0cd8ac2c4788f57cd9ae0a9c638d17cff8335c5d784e2a05cfd2393', '2026-08-04 17:55:00.330095+00');
INSERT INTO public.schema_migrations VALUES (45, '045_search_tag_trigger.sql', 'a8f8ba86025f7d5de746b862390507e0407a49546ed4ecd3a9bd71959af439cc', '2026-08-04 17:55:00.333952+00');
INSERT INTO public.schema_migrations VALUES (46, '046_user_schema_alignment.sql', '24c2609ee46cc177a120bb122c1bd2b72c112c4472fcd76607b54ddd4d9eb8d0', '2026-08-04 17:55:00.335692+00');
INSERT INTO public.schema_migrations VALUES (47, '047_pg_trgm_indexes.sql', '1370f7d9a607850b7a5167dac250cbdcc7ff33fe395b365ee1f203a1dbd745a8', '2026-08-04 17:55:00.341686+00');
INSERT INTO public.schema_migrations VALUES (48, '048_computed_columns.sql', 'b0f6a6f64bf51efd20e288eb3d529d54c556ddd3ecf07e6a45c865d4f10da898', '2026-08-04 17:55:00.342431+00');
INSERT INTO public.schema_migrations VALUES (49, '049_search_trigram_fallback.sql', '155e0765feaf8f1803b697bfe1e9bb4c50f918aed4879b040e834233a3040190', '2026-08-04 17:55:00.347072+00');
INSERT INTO public.schema_migrations VALUES (50, '050_verification_and_terms.sql', 'd6b7f269df4c16e9ef81d0e4e44363d85b22ed5162074e0a7da21f454bad96b8', '2026-08-04 17:55:00.347859+00');


--
-- Data for Name: tag_groups; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tag_suggestions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tags; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.tags VALUES (1, 'synthetic-a', 'seed', 1, '2026-08-04 17:55:00.416481+00', '2026-08-04 17:55:00.416481+00');
INSERT INTO public.tags VALUES (2, 'synthetic-b', 'seed', 2, '2026-08-04 17:55:00.416481+00', '2026-08-04 17:55:00.416481+00');
INSERT INTO public.tags VALUES (3, 'synthetic-c', 'seed', 3, '2026-08-04 17:55:00.416481+00', '2026-08-04 17:55:00.416481+00');


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.users VALUES (1, 'fixture-seed@example.invalid', '$2a$10$synthetic-hash-do-not-use', 'fixture-seed-user', NULL, NULL, 10, 'zh-CN', 'user', false, NULL, '{}', '2026-08-04 17:55:00.417415+00', '2026-08-04 17:55:00.417415+00', NULL, NULL, NULL, NULL, NULL);


--
-- Name: agent_conversations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.agent_conversations_id_seq', 1, false);


--
-- Name: agent_messages_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.agent_messages_id_seq', 1, false);


--
-- Name: ai_review_records_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.ai_review_records_id_seq', 1, false);


--
-- Name: appeals_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.appeals_id_seq', 1, false);


--
-- Name: browse_history_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.browse_history_id_seq', 1, false);


--
-- Name: categories_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.categories_id_seq', 17, true);


--
-- Name: comments_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.comments_id_seq', 1, false);


--
-- Name: content_attachments_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.content_attachments_id_seq', 1, false);


--
-- Name: content_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.content_items_id_seq', 1, false);


--
-- Name: content_versions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.content_versions_id_seq', 1, false);


--
-- Name: conversations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.conversations_id_seq', 1, false);


--
-- Name: discussions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.discussions_id_seq', 1, false);


--
-- Name: follows_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.follows_id_seq', 1, false);


--
-- Name: ip_review_logs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.ip_review_logs_id_seq', 1, false);


--
-- Name: ips_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.ips_id_seq', 1, false);


--
-- Name: judge_cases_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.judge_cases_id_seq', 1, false);


--
-- Name: judge_exam_records_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.judge_exam_records_id_seq', 1, false);


--
-- Name: judge_qualifications_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.judge_qualifications_id_seq', 1, false);


--
-- Name: judge_questions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.judge_questions_id_seq', 1, false);


--
-- Name: judge_reason_votes_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.judge_reason_votes_id_seq', 1, false);


--
-- Name: judge_votes_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.judge_votes_id_seq', 1, false);


--
-- Name: llm_configs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.llm_configs_id_seq', 1, false);


--
-- Name: messages_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.messages_id_seq', 1, false);


--
-- Name: notifications_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.notifications_id_seq', 1, false);


--
-- Name: oauth_accounts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.oauth_accounts_id_seq', 1, false);


--
-- Name: password_reset_tokens_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.password_reset_tokens_id_seq', 1, false);


--
-- Name: pull_requests_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.pull_requests_id_seq', 1, false);


--
-- Name: reactions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.reactions_id_seq', 1, false);


--
-- Name: rehab_completions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.rehab_completions_id_seq', 1, false);


--
-- Name: rehab_courses_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.rehab_courses_id_seq', 5, true);


--
-- Name: reports_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.reports_id_seq', 1, false);


--
-- Name: reputation_logs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.reputation_logs_id_seq', 1, false);


--
-- Name: saved_searches_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.saved_searches_id_seq', 1, false);


--
-- Name: schema_migration_attempts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.schema_migration_attempts_id_seq', 2, true);


--
-- Name: tag_groups_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.tag_groups_id_seq', 1, false);


--
-- Name: tag_suggestions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.tag_suggestions_id_seq', 1, false);


--
-- Name: tags_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.tags_id_seq', 3, true);


--
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.users_id_seq', 1, true);


--
-- Name: agent_conversations agent_conversations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_conversations
    ADD CONSTRAINT agent_conversations_pkey PRIMARY KEY (id);


--
-- Name: agent_messages agent_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_messages
    ADD CONSTRAINT agent_messages_pkey PRIMARY KEY (id);


--
-- Name: ai_review_records ai_review_records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_review_records
    ADD CONSTRAINT ai_review_records_pkey PRIMARY KEY (id);


--
-- Name: appeals appeals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.appeals
    ADD CONSTRAINT appeals_pkey PRIMARY KEY (id);


--
-- Name: author_blocklist author_blocklist_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.author_blocklist
    ADD CONSTRAINT author_blocklist_pkey PRIMARY KEY (author_id, blocked_id);


--
-- Name: browse_history browse_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browse_history
    ADD CONSTRAINT browse_history_pkey PRIMARY KEY (id);


--
-- Name: browse_history browse_history_user_id_content_item_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browse_history
    ADD CONSTRAINT browse_history_user_id_content_item_id_key UNIQUE (user_id, content_item_id);


--
-- Name: categories categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_pkey PRIMARY KEY (id);


--
-- Name: categories categories_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_slug_key UNIQUE (slug);


--
-- Name: comments comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (id);


--
-- Name: content_attachments content_attachments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_attachments
    ADD CONSTRAINT content_attachments_pkey PRIMARY KEY (id);


--
-- Name: content_contributors content_contributors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_contributors
    ADD CONSTRAINT content_contributors_pkey PRIMARY KEY (content_item_id, user_id);


--
-- Name: content_embeddings content_embeddings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_embeddings
    ADD CONSTRAINT content_embeddings_pkey PRIMARY KEY (content_item_id);


--
-- Name: content_items content_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_items
    ADD CONSTRAINT content_items_pkey PRIMARY KEY (id);


--
-- Name: content_tags content_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_tags
    ADD CONSTRAINT content_tags_pkey PRIMARY KEY (content_item_id, tag);


--
-- Name: content_versions content_versions_content_item_id_version_number_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT content_versions_content_item_id_version_number_key UNIQUE (content_item_id, version_number);


--
-- Name: content_versions content_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT content_versions_pkey PRIMARY KEY (id);


--
-- Name: conversation_participants conversation_participants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversation_participants
    ADD CONSTRAINT conversation_participants_pkey PRIMARY KEY (conversation_id, user_id);


--
-- Name: conversations conversations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversations
    ADD CONSTRAINT conversations_pkey PRIMARY KEY (id);


--
-- Name: discussions discussions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discussions
    ADD CONSTRAINT discussions_pkey PRIMARY KEY (id);


--
-- Name: favorites favorites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_pkey PRIMARY KEY (user_id, content_item_id);


--
-- Name: follows follows_follower_id_target_type_target_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_follower_id_target_type_target_id_key UNIQUE (follower_id, target_type, target_id);


--
-- Name: follows follows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_pkey PRIMARY KEY (id);


--
-- Name: ip_review_logs ip_review_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ip_review_logs
    ADD CONSTRAINT ip_review_logs_pkey PRIMARY KEY (id);


--
-- Name: ip_tags ip_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ip_tags
    ADD CONSTRAINT ip_tags_pkey PRIMARY KEY (ip_id, tag);


--
-- Name: ips ips_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ips
    ADD CONSTRAINT ips_pkey PRIMARY KEY (id);


--
-- Name: ips ips_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ips
    ADD CONSTRAINT ips_slug_key UNIQUE (slug);


--
-- Name: judge_cases judge_cases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_cases
    ADD CONSTRAINT judge_cases_pkey PRIMARY KEY (id);


--
-- Name: judge_exam_records judge_exam_records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_exam_records
    ADD CONSTRAINT judge_exam_records_pkey PRIMARY KEY (id);


--
-- Name: judge_qualifications judge_qualifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_qualifications
    ADD CONSTRAINT judge_qualifications_pkey PRIMARY KEY (id);


--
-- Name: judge_qualifications judge_qualifications_user_id_content_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_qualifications
    ADD CONSTRAINT judge_qualifications_user_id_content_type_key UNIQUE (user_id, content_type);


--
-- Name: judge_questions judge_questions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_questions
    ADD CONSTRAINT judge_questions_pkey PRIMARY KEY (id);


--
-- Name: judge_reason_votes judge_reason_votes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_reason_votes
    ADD CONSTRAINT judge_reason_votes_pkey PRIMARY KEY (id);


--
-- Name: judge_reason_votes judge_reason_votes_reason_owner_vote_id_voter_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_reason_votes
    ADD CONSTRAINT judge_reason_votes_reason_owner_vote_id_voter_id_key UNIQUE (reason_owner_vote_id, voter_id);


--
-- Name: judge_votes judge_votes_case_id_judge_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_votes
    ADD CONSTRAINT judge_votes_case_id_judge_id_key UNIQUE (case_id, judge_id);


--
-- Name: judge_votes judge_votes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_votes
    ADD CONSTRAINT judge_votes_pkey PRIMARY KEY (id);


--
-- Name: llm_configs llm_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_configs
    ADD CONSTRAINT llm_configs_pkey PRIMARY KEY (id);


--
-- Name: messages messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_pkey PRIMARY KEY (id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: oauth_accounts oauth_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_accounts
    ADD CONSTRAINT oauth_accounts_pkey PRIMARY KEY (id);


--
-- Name: oauth_accounts oauth_accounts_provider_provider_uid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_accounts
    ADD CONSTRAINT oauth_accounts_provider_provider_uid_key UNIQUE (provider, provider_uid);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_token_key UNIQUE (token);


--
-- Name: pull_requests pull_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests
    ADD CONSTRAINT pull_requests_pkey PRIMARY KEY (id);


--
-- Name: reactions reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT reactions_pkey PRIMARY KEY (id);


--
-- Name: reactions reactions_user_id_target_type_target_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT reactions_user_id_target_type_target_id_key UNIQUE (user_id, target_type, target_id);


--
-- Name: rehab_completions rehab_completions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rehab_completions
    ADD CONSTRAINT rehab_completions_pkey PRIMARY KEY (id);


--
-- Name: rehab_completions rehab_completions_user_id_course_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rehab_completions
    ADD CONSTRAINT rehab_completions_user_id_course_id_key UNIQUE (user_id, course_id);


--
-- Name: rehab_courses rehab_courses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rehab_courses
    ADD CONSTRAINT rehab_courses_pkey PRIMARY KEY (id);


--
-- Name: rehab_courses rehab_courses_violation_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rehab_courses
    ADD CONSTRAINT rehab_courses_violation_type_key UNIQUE (violation_type);


--
-- Name: reports reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_pkey PRIMARY KEY (id);


--
-- Name: reports reports_unique_report; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_unique_report UNIQUE (reporter_id, target_type, target_id);


--
-- Name: reputation_logs reputation_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reputation_logs
    ADD CONSTRAINT reputation_logs_pkey PRIMARY KEY (id);


--
-- Name: saved_searches saved_searches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_searches
    ADD CONSTRAINT saved_searches_pkey PRIMARY KEY (id);


--
-- Name: schema_migration_attempts schema_migration_attempts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migration_attempts
    ADD CONSTRAINT schema_migration_attempts_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_filename_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_filename_key UNIQUE (filename);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: tag_groups tag_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_groups
    ADD CONSTRAINT tag_groups_pkey PRIMARY KEY (id);


--
-- Name: tag_suggestions tag_suggestions_content_item_id_user_id_tag_action_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_suggestions
    ADD CONSTRAINT tag_suggestions_content_item_id_user_id_tag_action_key UNIQUE (content_item_id, user_id, tag, action);


--
-- Name: tag_suggestions tag_suggestions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_suggestions
    ADD CONSTRAINT tag_suggestions_pkey PRIMARY KEY (id);


--
-- Name: tags tags_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_name_key UNIQUE (name);


--
-- Name: tags tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: idx_agent_conversations_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_conversations_user ON public.agent_conversations USING btree (user_id, created_at DESC);


--
-- Name: idx_agent_messages_conv; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_messages_conv ON public.agent_messages USING btree (conversation_id, created_at);


--
-- Name: idx_ai_review_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_review_target ON public.ai_review_records USING btree (target_type, target_id, scanned_at DESC);


--
-- Name: idx_appeals_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_status ON public.appeals USING btree (status);


--
-- Name: idx_appeals_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_user ON public.appeals USING btree (user_id);


--
-- Name: idx_browse_history_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_browse_history_user ON public.browse_history USING btree (user_id, viewed_at DESC);


--
-- Name: idx_browse_history_user_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_browse_history_user_time ON public.browse_history USING btree (user_id, viewed_at DESC);


--
-- Name: idx_browse_history_user_viewed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_browse_history_user_viewed ON public.browse_history USING btree (user_id, viewed_at DESC);


--
-- Name: idx_categories_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_categories_parent ON public.categories USING btree (parent_id);


--
-- Name: idx_categories_zone_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_categories_zone_level ON public.categories USING btree (zone, level, sort_order);


--
-- Name: idx_comments_author; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comments_author ON public.comments USING btree (author_id);


--
-- Name: idx_comments_content; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comments_content ON public.comments USING btree (content_item_id);


--
-- Name: idx_comments_discussion; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comments_discussion ON public.comments USING btree (discussion_id);


--
-- Name: idx_comments_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comments_parent ON public.comments USING btree (parent_id);


--
-- Name: idx_comments_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comments_target ON public.comments USING btree (target_type, target_id);


--
-- Name: idx_content_attachments_item; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_attachments_item ON public.content_attachments USING btree (content_item_id);


--
-- Name: idx_content_embeddings_ivfflat; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_embeddings_ivfflat ON public.content_embeddings USING ivfflat (embedding public.vector_cosine_ops) WITH (lists='100');


--
-- Name: idx_content_items_author; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_author ON public.content_items USING btree (author_id);


--
-- Name: idx_content_items_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_category ON public.content_items USING btree (category);


--
-- Name: idx_content_items_hot_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_hot_score ON public.content_items USING btree (hot_score DESC NULLS LAST);


--
-- Name: idx_content_items_ip; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_ip ON public.content_items USING btree (ip_id);


--
-- Name: idx_content_items_rating_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_rating_score ON public.content_items USING btree (rating_score DESC NULLS LAST);


--
-- Name: idx_content_items_search_vector; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_search_vector ON public.content_items USING gin (search_vector);


--
-- Name: idx_content_items_source_original; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_source_original ON public.content_items USING btree (source_original_id, status, created_at DESC) WHERE (source_original_id IS NOT NULL);


--
-- Name: idx_content_items_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_status ON public.content_items USING btree (status);


--
-- Name: idx_content_items_title_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_title_trgm ON public.content_items USING gin (title public.gin_trgm_ops);


--
-- Name: idx_content_items_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_type ON public.content_items USING btree (content_type);


--
-- Name: idx_content_items_zone; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_zone ON public.content_items USING btree (zone);


--
-- Name: idx_content_items_zone_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_items_zone_status ON public.content_items USING btree (zone, status);


--
-- Name: idx_content_tags_tag_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_tags_tag_trgm ON public.content_tags USING gin (tag public.gin_trgm_ops);


--
-- Name: idx_conv_participants_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conv_participants_user ON public.conversation_participants USING btree (user_id);


--
-- Name: idx_discussions_author; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_discussions_author ON public.discussions USING btree (author_id);


--
-- Name: idx_discussions_ip; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_discussions_ip ON public.discussions USING btree (ip_id, updated_at DESC);


--
-- Name: idx_discussions_ip_last_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_discussions_ip_last_active ON public.discussions USING btree (ip_id, last_active_at DESC);


--
-- Name: idx_discussions_search; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_discussions_search ON public.discussions USING gin (to_tsvector('simple'::regconfig, (((title)::text || ' '::text) || COALESCE(body, ''::text))));


--
-- Name: idx_follows_follower; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_follows_follower ON public.follows USING btree (follower_id);


--
-- Name: idx_follows_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_follows_target ON public.follows USING btree (target_type, target_id);


--
-- Name: idx_follows_target_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_follows_target_created ON public.follows USING btree (target_type, target_id, created_at DESC);


--
-- Name: idx_ip_review_logs_ip; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ip_review_logs_ip ON public.ip_review_logs USING btree (ip_id, created_at DESC);


--
-- Name: idx_ips_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ips_category ON public.ips USING btree (category);


--
-- Name: idx_ips_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ips_name ON public.ips USING gin (to_tsvector('simple'::regconfig, (name)::text));


--
-- Name: idx_ips_name_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ips_name_trgm ON public.ips USING gin (name public.gin_trgm_ops);


--
-- Name: idx_ips_popularity_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ips_popularity_score ON public.ips USING btree (popularity_score DESC NULLS LAST);


--
-- Name: idx_ips_search_vector; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ips_search_vector ON public.ips USING gin (search_vector);


--
-- Name: idx_ips_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ips_status ON public.ips USING btree (status);


--
-- Name: idx_judge_cases_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_judge_cases_status ON public.judge_cases USING btree (status, created_at DESC);


--
-- Name: idx_judge_cases_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_judge_cases_target ON public.judge_cases USING btree (target_type, target_id);


--
-- Name: idx_judge_exam_user_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_judge_exam_user_type ON public.judge_exam_records USING btree (user_id, content_type, taken_at DESC);


--
-- Name: idx_judge_qual_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_judge_qual_type ON public.judge_qualifications USING btree (content_type, is_active);


--
-- Name: idx_judge_qual_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_judge_qual_user ON public.judge_qualifications USING btree (user_id);


--
-- Name: idx_judge_questions_type_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_judge_questions_type_active ON public.judge_questions USING btree (content_type, is_active);


--
-- Name: idx_judge_reason_votes_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_judge_reason_votes_owner ON public.judge_reason_votes USING btree (reason_owner_vote_id);


--
-- Name: idx_judge_reason_votes_vote; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_judge_reason_votes_vote ON public.judge_reason_votes USING btree (reason_owner_vote_id);


--
-- Name: idx_llm_configs_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_llm_configs_active ON public.llm_configs USING btree (is_active) WHERE (is_active = true);


--
-- Name: idx_messages_conv_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_messages_conv_created ON public.messages USING btree (conversation_id, created_at DESC);


--
-- Name: idx_notifications_user_channel; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_channel ON public.notifications USING btree (user_id, channel, created_at DESC);


--
-- Name: idx_notifications_user_unread; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_unread ON public.notifications USING btree (user_id, is_read, created_at DESC);


--
-- Name: idx_oauth_accounts_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_oauth_accounts_user ON public.oauth_accounts USING btree (user_id);


--
-- Name: idx_password_reset_tokens_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_password_reset_tokens_token ON public.password_reset_tokens USING btree (token);


--
-- Name: idx_password_reset_tokens_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_password_reset_tokens_user ON public.password_reset_tokens USING btree (user_id);


--
-- Name: idx_pr_content; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_pr_content ON public.pull_requests USING btree (content_item_id, status);


--
-- Name: idx_pr_submitter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_pr_submitter ON public.pull_requests USING btree (submitter_id);


--
-- Name: idx_reactions_user_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reactions_user_type ON public.reactions USING btree (user_id, target_type, reaction);


--
-- Name: idx_rehab_completions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rehab_completions_user ON public.rehab_completions USING btree (user_id);


--
-- Name: idx_reports_reporter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reports_reporter ON public.reports USING btree (reporter_id);


--
-- Name: idx_reports_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reports_status ON public.reports USING btree (status, created_at DESC);


--
-- Name: idx_reports_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reports_target ON public.reports USING btree (target_type, target_id);


--
-- Name: idx_reputation_logs_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reputation_logs_user ON public.reputation_logs USING btree (user_id, created_at DESC);


--
-- Name: idx_saved_searches_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_searches_user ON public.saved_searches USING btree (user_id);


--
-- Name: idx_schema_migration_attempts_version_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_schema_migration_attempts_version_status ON public.schema_migration_attempts USING btree (version, status);


--
-- Name: idx_tag_groups_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tag_groups_user ON public.tag_groups USING btree (user_id);


--
-- Name: idx_tag_suggestions_content; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tag_suggestions_content ON public.tag_suggestions USING btree (content_item_id, status);


--
-- Name: idx_tag_suggestions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tag_suggestions_user ON public.tag_suggestions USING btree (user_id);


--
-- Name: idx_tags_category_usage; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tags_category_usage ON public.tags USING btree (category, usage_count DESC);


--
-- Name: idx_tags_name_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tags_name_gin ON public.tags USING gin (to_tsvector('simple'::regconfig, (name)::text));


--
-- Name: idx_tags_name_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tags_name_trgm ON public.tags USING gin (name public.gin_trgm_ops);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_email ON public.users USING btree (email);


--
-- Name: idx_users_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_role ON public.users USING btree (role);


--
-- Name: idx_users_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_username ON public.users USING btree (username);


--
-- Name: idx_users_username_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_username_trgm ON public.users USING gin (username public.gin_trgm_ops);


--
-- Name: idx_versions_content; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_versions_content ON public.content_versions USING btree (content_item_id);


--
-- Name: idx_versions_content_latest; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_versions_content_latest ON public.content_versions USING btree (content_item_id) WHERE (is_latest = true);


--
-- Name: idx_versions_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_versions_status ON public.content_versions USING btree (status);


--
-- Name: content_items content_items_search_vector_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER content_items_search_vector_trigger BEFORE INSERT OR UPDATE OF title, description ON public.content_items FOR EACH ROW EXECUTE FUNCTION public.content_items_search_vector_update();


--
-- Name: ips ips_search_vector_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER ips_search_vector_trigger BEFORE INSERT OR UPDATE OF name, description ON public.ips FOR EACH ROW EXECUTE FUNCTION public.ips_search_vector_update();


--
-- Name: content_tags trg_content_tags_search_vector; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_content_tags_search_vector AFTER INSERT OR DELETE OR UPDATE ON public.content_tags FOR EACH ROW EXECUTE FUNCTION public.content_tags_search_vector_update();


--
-- Name: ip_tags trg_ip_tags_search_vector; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_ip_tags_search_vector AFTER INSERT OR DELETE OR UPDATE ON public.ip_tags FOR EACH ROW EXECUTE FUNCTION public.ip_tags_search_vector_update();


--
-- Name: agent_conversations agent_conversations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_conversations
    ADD CONSTRAINT agent_conversations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: agent_messages agent_messages_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_messages
    ADD CONSTRAINT agent_messages_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.agent_conversations(id) ON DELETE CASCADE;


--
-- Name: appeals appeals_resolved_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.appeals
    ADD CONSTRAINT appeals_resolved_by_fkey FOREIGN KEY (resolved_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: appeals appeals_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.appeals
    ADD CONSTRAINT appeals_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: author_blocklist author_blocklist_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.author_blocklist
    ADD CONSTRAINT author_blocklist_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: author_blocklist author_blocklist_blocked_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.author_blocklist
    ADD CONSTRAINT author_blocklist_blocked_id_fkey FOREIGN KEY (blocked_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: browse_history browse_history_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browse_history
    ADD CONSTRAINT browse_history_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: browse_history browse_history_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browse_history
    ADD CONSTRAINT browse_history_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: categories categories_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.categories(id);


--
-- Name: comments comments_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: comments comments_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: comments comments_discussion_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_discussion_id_fkey FOREIGN KEY (discussion_id) REFERENCES public.discussions(id) ON DELETE CASCADE;


--
-- Name: comments comments_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.comments(id) ON DELETE CASCADE;


--
-- Name: content_attachments content_attachments_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_attachments
    ADD CONSTRAINT content_attachments_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: content_contributors content_contributors_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_contributors
    ADD CONSTRAINT content_contributors_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: content_contributors content_contributors_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_contributors
    ADD CONSTRAINT content_contributors_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: content_embeddings content_embeddings_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_embeddings
    ADD CONSTRAINT content_embeddings_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: content_items content_items_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_items
    ADD CONSTRAINT content_items_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: content_items content_items_ip_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_items
    ADD CONSTRAINT content_items_ip_id_fkey FOREIGN KEY (ip_id) REFERENCES public.ips(id) ON DELETE SET NULL;


--
-- Name: content_items content_items_source_original_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_items
    ADD CONSTRAINT content_items_source_original_id_fkey FOREIGN KEY (source_original_id) REFERENCES public.content_items(id) ON DELETE SET NULL;


--
-- Name: content_tags content_tags_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_tags
    ADD CONSTRAINT content_tags_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: content_versions content_versions_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT content_versions_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: content_versions content_versions_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT content_versions_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: content_versions content_versions_parent_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT content_versions_parent_version_id_fkey FOREIGN KEY (parent_version_id) REFERENCES public.content_versions(id) ON DELETE SET NULL;


--
-- Name: conversation_participants conversation_participants_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversation_participants
    ADD CONSTRAINT conversation_participants_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: conversation_participants conversation_participants_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversation_participants
    ADD CONSTRAINT conversation_participants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: discussions discussions_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discussions
    ADD CONSTRAINT discussions_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: discussions discussions_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discussions
    ADD CONSTRAINT discussions_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: discussions discussions_ip_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discussions
    ADD CONSTRAINT discussions_ip_id_fkey FOREIGN KEY (ip_id) REFERENCES public.ips(id) ON DELETE CASCADE;


--
-- Name: favorites favorites_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: favorites favorites_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: follows follows_follower_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_follower_id_fkey FOREIGN KEY (follower_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: ip_review_logs ip_review_logs_ip_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ip_review_logs
    ADD CONSTRAINT ip_review_logs_ip_id_fkey FOREIGN KEY (ip_id) REFERENCES public.ips(id) ON DELETE CASCADE;


--
-- Name: ip_review_logs ip_review_logs_reviewer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ip_review_logs
    ADD CONSTRAINT ip_review_logs_reviewer_id_fkey FOREIGN KEY (reviewer_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: ip_tags ip_tags_ip_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ip_tags
    ADD CONSTRAINT ip_tags_ip_id_fkey FOREIGN KEY (ip_id) REFERENCES public.ips(id) ON DELETE CASCADE;


--
-- Name: ips ips_creator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ips
    ADD CONSTRAINT ips_creator_id_fkey FOREIGN KEY (creator_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: judge_exam_records judge_exam_records_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_exam_records
    ADD CONSTRAINT judge_exam_records_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: judge_qualifications judge_qualifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_qualifications
    ADD CONSTRAINT judge_qualifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: judge_reason_votes judge_reason_votes_reason_owner_vote_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_reason_votes
    ADD CONSTRAINT judge_reason_votes_reason_owner_vote_id_fkey FOREIGN KEY (reason_owner_vote_id) REFERENCES public.judge_votes(id) ON DELETE CASCADE;


--
-- Name: judge_reason_votes judge_reason_votes_voter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_reason_votes
    ADD CONSTRAINT judge_reason_votes_voter_id_fkey FOREIGN KEY (voter_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: judge_votes judge_votes_case_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_votes
    ADD CONSTRAINT judge_votes_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.judge_cases(id) ON DELETE CASCADE;


--
-- Name: judge_votes judge_votes_judge_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.judge_votes
    ADD CONSTRAINT judge_votes_judge_id_fkey FOREIGN KEY (judge_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: messages messages_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: messages messages_sender_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_sender_id_fkey FOREIGN KEY (sender_id) REFERENCES public.users(id);


--
-- Name: notifications notifications_sender_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_sender_id_fkey FOREIGN KEY (sender_id) REFERENCES public.users(id);


--
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: oauth_accounts oauth_accounts_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_accounts
    ADD CONSTRAINT oauth_accounts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: password_reset_tokens password_reset_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: pull_requests pull_requests_base_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests
    ADD CONSTRAINT pull_requests_base_version_id_fkey FOREIGN KEY (base_version_id) REFERENCES public.content_versions(id);


--
-- Name: pull_requests pull_requests_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests
    ADD CONSTRAINT pull_requests_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: pull_requests pull_requests_proposed_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests
    ADD CONSTRAINT pull_requests_proposed_version_id_fkey FOREIGN KEY (proposed_version_id) REFERENCES public.content_versions(id);


--
-- Name: pull_requests pull_requests_submitter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests
    ADD CONSTRAINT pull_requests_submitter_id_fkey FOREIGN KEY (submitter_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: reactions reactions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT reactions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: rehab_completions rehab_completions_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rehab_completions
    ADD CONSTRAINT rehab_completions_course_id_fkey FOREIGN KEY (course_id) REFERENCES public.rehab_courses(id);


--
-- Name: rehab_completions rehab_completions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rehab_completions
    ADD CONSTRAINT rehab_completions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: reports reports_reporter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_reporter_id_fkey FOREIGN KEY (reporter_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: reputation_logs reputation_logs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reputation_logs
    ADD CONSTRAINT reputation_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: saved_searches saved_searches_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_searches
    ADD CONSTRAINT saved_searches_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: tag_groups tag_groups_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_groups
    ADD CONSTRAINT tag_groups_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: tag_suggestions tag_suggestions_content_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_suggestions
    ADD CONSTRAINT tag_suggestions_content_item_id_fkey FOREIGN KEY (content_item_id) REFERENCES public.content_items(id) ON DELETE CASCADE;


--
-- Name: tag_suggestions tag_suggestions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_suggestions
    ADD CONSTRAINT tag_suggestions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- PostgreSQL database dump complete
--

\unrestrict q5RFoRmfNOHibj2BqLpPe9IbruLOgFdzs4pFBrwkGeb7ME5fFIu6eBg2RdPRFmy

