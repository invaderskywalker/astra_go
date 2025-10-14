/* eslint-disable @typescript-eslint/no-explicit-any */
import React, { useEffect, useState } from 'react';
import '../styles/learning.css';
import { fetchLearnings } from "../api";

interface LearningKnowledge {
  id: string;
  knowledge_type: string;
  knowledge_blob?: string;
  content?: string;
  created_at: string;
  user_id: number;
}

interface LearningListProps {
  token: string;
  userId: number;
}

const KNOWLEDGE_TYPES = [
  'all',
  'concept',
  'code_fact',
  'mental_model',
  'workflow',
  'reference',
  'other'
];

const LearningList: React.FC<LearningListProps> = ({ token, userId }) => {
  const [learnings, setLearnings] = useState<LearningKnowledge[]>([]);
  const [filterType, setFilterType] = useState('all');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleFetchLearnings = async (type: string) => {
    setIsLoading(true);
    try {
      setError(null);
      const data = await fetchLearnings(token, userId, type);
      setLearnings(data);
    } catch (e: any) {
      setError(e.message || 'Failed to load learnings');
      setLearnings([]);
    } finally {
      setIsLoading(false);
    }
  };
  useEffect(() => {
    handleFetchLearnings(filterType);
  }, [filterType, userId]);

  return (
    <div className="learning-list-container">
      <div className="learning-list-header">
        <h2>Your Learnings</h2>
        <select
          className="learning-type-filter"
          value={filterType}
          onChange={e => setFilterType(e.target.value)}
        >
          {KNOWLEDGE_TYPES.map(type => (
            <option value={type} key={type}>{type === 'all' ? 'All Types' : type.replace('_', ' ')}</option>
          ))}
        </select>
      </div>
      {isLoading && <div className="learning-list-loading">Loadingâ¦</div>}
      {error && <div className="learning-list-error">{error}</div>}
      <div className="learning-list-items">
        {learnings.length === 0 && !isLoading && !error && (
          <div className="learning-list-empty">No learnings found for this filter.</div>
        )}
        {learnings.map(lk => (
          <div className="learning-item" key={lk.id}>
            <div className="learning-type-tag">{lk.knowledge_type}</div>
            <div className="learning-content">{lk.knowledge_blob || lk.content}</div>
            <div className="learning-meta">
              <span className="learning-date">{new Date(lk.created_at).toLocaleString()}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default LearningList;
